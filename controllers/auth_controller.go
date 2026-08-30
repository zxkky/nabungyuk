package controllers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/models"
)

// AuthController handles authentication-related requests
type AuthController struct{}

// NewAuthController creates a new AuthController
func NewAuthController() *AuthController {
	return &AuthController{}
}

// RegisterRequest is the request body for registration
type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest is the request body for login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register handles user registration
func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Validasi gagal: " + validationMessage(err),
		})
		return
	}

	// Normalize and validate before querying/creating.
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if len([]rune(req.Name)) < 2 || len([]rune(req.Name)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nama harus 2-100 karakter"})
		return
	}
	if len(req.Password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Password maksimal 72 karakter"})
		return
	}

	// Check if user already exists
	var existing models.User
	if err := config.DB.Where("email = ?", strings.ToLower(req.Email)).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Email sudah terdaftar",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal meng-hash password",
		})
		return
	}

	// Create user
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
	}
	if err := config.DB.Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Email sudah terdaftar"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal membuat user"})
		return
	}

	// Generate token
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membuat token",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Registrasi berhasil",
		"data": gin.H{
			"user":  user.PublicUser(),
			"token": token,
		},
	})
}

// Login handles user login
func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Email dan password wajib diisi",
		})
		return
	}

	// Find user by email
	var user models.User
	if err := config.DB.Where("email = ?", strings.ToLower(req.Email)).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Email atau password salah",
		})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Email atau password salah",
		})
		return
	}

	// Generate token
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membuat token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Login berhasil",
		"data": gin.H{
			"user":  user.PublicUser(),
			"token": token,
		},
	})
}

// Logout handles user logout
// JWT is stateless; client should discard the token.
func (ac *AuthController) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout berhasil",
	})
}

// GetProfile returns the authenticated user's profile
func (ac *AuthController) GetProfile(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "User tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile berhasil diambil",
		"data": gin.H{
			"user": user.PublicUser(),
		},
	})
}

// generateToken creates a JWT token for a user
func generateToken(userID uint) (string, error) {
	secret := middleware.JWTSecret
	claims := jwt.MapClaims{
		"user_id": userID,
		"iss":     "nabungyuk",
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// validationMessage converts validator errors to a readable message
func validationMessage(err error) string {
	msg := err.Error()
	// Strip technical details, keep first sentence
	if idx := strings.Index(msg, "\n"); idx != -1 {
		msg = msg[:idx]
	}
	return msg
}
