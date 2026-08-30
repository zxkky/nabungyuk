package middleware

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is the secret key for JWT signing.
var JWTSecret []byte

var knownInsecureSecrets = map[string]bool{
	"": true,
	"nabungyuk-dev-secret-change-in-production": true,
	"your_jwt_secret_key_here":                  true,
	"secret":                                    true,
	"jwt_secret":                                true,
}

func LoadJWTSecret() {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if knownInsecureSecrets[secret] || len(secret) < 32 {
		log.Fatal("FATAL: JWT_SECRET harus diisi dan minimal 32 karakter.")
	}
	JWTSecret = []byte(secret)
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Token not provided"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return JWTSecret, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid token claims"})
			c.Abort()
			return
		}

		userID, ok := getUserIDFromClaims(claims)
		if !ok || userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid user ID in token"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

func getUserIDFromClaims(claims jwt.MapClaims) (uint, bool) {
	v, ok := claims["user_id"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n <= 0 || n != float64(uint64(n)) || n > float64(^uint(0)) {
			return 0, false
		}
		return uint(n), true
	case string:
		id, err := strconv.ParseUint(n, 10, 0)
		return uint(id), err == nil && id > 0
	case uint:
		return n, n > 0
	case int64:
		return uint(n), n > 0
	default:
		return 0, false
	}
}

func extractToken(c *gin.Context) string {
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if authorization == "" {
		return ""
	}
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok && id > 0
}
