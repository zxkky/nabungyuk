package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/models"
)

// SavingController handles saving goal-related requests
type SavingController struct{}

// NewSavingController creates a new SavingController
func NewSavingController() *SavingController {
	return &SavingController{}
}

// savingGoalResponse is the JSON response shape for a saving goal
type savingGoalResponse struct {
	ID              uint      `json:"id"`
	Name            string    `json:"name"`
	TargetAmount    int64     `json:"target_amount"`
	Deadline        time.Time `json:"deadline"`
	Icon            string    `json:"icon"`
	CurrentAmount   int64     `json:"current_amount"`
	RemainingAmount int64     `json:"remaining_amount"`
	Progress        float64   `json:"progress"`
	DaysLeft        int64     `json:"days_left"`
	IsCompleted     bool      `json:"is_completed"`
	CreatedAt       time.Time `json:"created_at"`
}

// savingDepositResponse is the JSON response shape for a deposit
type savingDepositResponse struct {
	ID           uint      `json:"id"`
	SavingGoalID uint      `json:"saving_goal_id"`
	Amount       int64     `json:"amount"`
	Note         string    `json:"note"`
	DepositDate  time.Time `json:"deposit_date"`
	CreatedAt    time.Time `json:"created_at"`
}

// depositRequest is the request body for adding a deposit
type depositRequest struct {
	Amount      int64  `json:"amount" binding:"required"`
	Note        string `json:"note"`
	DepositDate string `json:"deposit_date"`
}

// GetAllSavingGoals returns all saving goals with progress for the user
func (sc *SavingController) GetAllSavingGoals(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	var goals []models.SavingGoal
	if err := config.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&goals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengambil target tabungan"})
		return
	}

	responses := make([]savingGoalResponse, len(goals))
	for i, g := range goals {
		current := sumDeposits(g.ID)
		responses[i] = toSavingGoalResponse(g, current)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Target tabungan berhasil diambil",
		"data": gin.H{
			"saving_goals": responses,
		},
	})
}

// GetSavingGoal returns a specific saving goal with progress and deposits
func (sc *SavingController) GetSavingGoal(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var goal models.SavingGoal
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&goal).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
		return
	}

	current := sumDeposits(goal.ID)

	// Fetch deposits
	var deposits []models.SavingDeposit
	config.DB.Where("saving_goal_id = ?", goal.ID).Order("deposit_date DESC").Find(&deposits)

	depositResponses := make([]savingDepositResponse, len(deposits))
	for i, d := range deposits {
		depositResponses[i] = toSavingDepositResponse(d)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Target tabungan berhasil diambil",
		"data": gin.H{
			"saving_goal": toSavingGoalResponse(goal, current),
			"deposits":    depositResponses,
		},
	})
}

// CreateSavingGoal creates a new saving goal
func (sc *SavingController) CreateSavingGoal(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required"`
		TargetAmount int64  `json:"target_amount" binding:"required"`
		Deadline     string `json:"deadline" binding:"required"`
		Icon         string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Validasi gagal: nama, target_amount, dan deadline wajib diisi"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len([]rune(req.Name)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nama target tabungan wajib diisi dan maksimal 100 karakter"})
		return
	}
	if req.TargetAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Target nominal harus lebih dari 0"})
		return
	}

	deadline, err := time.Parse("2006-01-02", req.Deadline)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Deadline tidak valid, gunakan format YYYY-MM-DD"})
		return
	}
	today := time.Now().In(time.Local)
	startToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	if deadline.Before(startToday) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Deadline tidak boleh sebelum hari ini"})
		return
	}

	icon := req.Icon
	if icon == "" {
		icon = models.IconDefault
	}
	if !containsString(models.GetIcons(), icon) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Icon target tidak valid"})
		return
	}

	goal := models.SavingGoal{
		UserID:       userID,
		Name:         req.Name,
		TargetAmount: req.TargetAmount,
		Deadline:     deadline,
		Icon:         icon,
	}
	if err := config.DB.Create(&goal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal membuat target tabungan: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Target tabungan berhasil dibuat",
		"data":    toSavingGoalResponse(goal, 0),
	})
}

// UpdateSavingGoal updates a saving goal (only if owned by user)
func (sc *SavingController) UpdateSavingGoal(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var goal models.SavingGoal
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&goal).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
		return
	}

	var body struct {
		Name         *string `json:"name"`
		TargetAmount *int64  `json:"target_amount"`
		Deadline     *string `json:"deadline"`
		Icon         *string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Body JSON tidak valid"})
		return
	}

	hasUpdates := false
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len([]rune(name)) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nama target tabungan wajib diisi dan maksimal 100 karakter"})
			return
		}
		goal.Name = name
		hasUpdates = true
	}
	if body.TargetAmount != nil {
		if *body.TargetAmount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Target nominal harus lebih dari 0"})
			return
		}
		goal.TargetAmount = *body.TargetAmount
		hasUpdates = true
	}
	if body.Deadline != nil && *body.Deadline != "" {
		parsed, err := time.Parse("2006-01-02", *body.Deadline)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Deadline tidak valid"})
			return
		}
		today := time.Now().In(time.Local)
		startToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
		if parsed.Before(startToday) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Deadline tidak boleh sebelum hari ini"})
			return
		}
		goal.Deadline = parsed
		hasUpdates = true
	}
	if body.Icon != nil {
		if !containsString(models.GetIcons(), *body.Icon) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Icon target tidak valid"})
			return
		}
		goal.Icon = *body.Icon
		hasUpdates = true
	}

	if !hasUpdates {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tidak ada field yang diupdate"})
		return
	}

	if err := config.DB.Save(&goal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengupdate target tabungan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Target tabungan berhasil diupdate",
		"data":    toSavingGoalResponse(goal, sumDeposits(goal.ID)),
	})
}

// DeleteSavingGoal deletes a saving goal (only if owned by user)
// Deposits are cascade-deleted by DB foreign key
func (sc *SavingController) DeleteSavingGoal(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var goal models.SavingGoal
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&goal).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
		return
	}

	if err := config.DB.Delete(&goal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal menghapus target tabungan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Target tabungan berhasil dihapus",
	})
}

// AddDeposit adds a deposit to a saving goal (only if owned by user)
func (sc *SavingController) AddDeposit(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var goal models.SavingGoal
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&goal).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
		return
	}

	var req depositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nominal setoran wajib diisi"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nominal setoran harus lebih dari 0"})
		return
	}

	depositDate := time.Now()
	if req.DepositDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DepositDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tanggal setoran tidak valid, gunakan format YYYY-MM-DD"})
			return
		}
		depositDate = parsed
	}

	if len([]rune(req.Note)) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Catatan terlalu panjang"})
		return
	}

	var deposit models.SavingDeposit
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		deposit = models.SavingDeposit{
			SavingGoalID: goal.ID,
			Amount:       req.Amount,
			Note:         strings.TrimSpace(req.Note),
			DepositDate:  depositDate,
		}
		return tx.Create(&deposit).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal menyimpan setoran"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Setoran berhasil ditambahkan",
		"data": gin.H{
			"deposit":     toSavingDepositResponse(deposit),
			"saving_goal": toSavingGoalResponse(goal, sumDeposits(goal.ID)),
		},
	})
}

// GetDeposits returns all deposits for a saving goal (only if owned by user)
func (sc *SavingController) GetDeposits(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var goal models.SavingGoal
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&goal).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
		return
	}

	var deposits []models.SavingDeposit
	if err := config.DB.Where("saving_goal_id = ?", goal.ID).Order("deposit_date DESC").Find(&deposits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengambil setoran"})
		return
	}

	responses := make([]savingDepositResponse, len(deposits))
	for i, d := range deposits {
		responses[i] = toSavingDepositResponse(d)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Setoran berhasil diambil",
		"data": gin.H{
			"deposits": responses,
		},
	})
}

// --- Helpers ---

// sumDeposits computes the total amount of deposits for a goal
func sumDeposits(goalID uint) int64 {
	var sum int64
	config.DB.Model(&models.SavingDeposit{}).
		Where("saving_goal_id = ?", goalID).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum)
	return sum
}

// toSavingGoalResponse builds the response shape with computed progress & estimation
func toSavingGoalResponse(g models.SavingGoal, current int64) savingGoalResponse {
	progress := 0.0
	if g.TargetAmount > 0 {
		progress = float64(current) / float64(g.TargetAmount) * 100
	}
	if progress > 100 {
		progress = 100
	}

	remaining := g.TargetAmount - current
	if remaining < 0 {
		remaining = 0
	}

	daysLeft := int64(0)
	if !g.Deadline.IsZero() {
		daysLeft = int64(time.Until(g.Deadline).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
	}

	return savingGoalResponse{
		ID:              g.ID,
		Name:            g.Name,
		TargetAmount:    g.TargetAmount,
		Deadline:        g.Deadline,
		Icon:            g.Icon,
		CurrentAmount:   current,
		RemainingAmount: remaining,
		Progress:        progress,
		DaysLeft:        daysLeft,
		IsCompleted:     current >= g.TargetAmount,
		CreatedAt:       g.CreatedAt,
	}
}

// toSavingDepositResponse converts a deposit to the response shape
func toSavingDepositResponse(d models.SavingDeposit) savingDepositResponse {
	return savingDepositResponse{
		ID:           d.ID,
		SavingGoalID: d.SavingGoalID,
		Amount:       d.Amount,
		Note:         d.Note,
		DepositDate:  d.DepositDate,
		CreatedAt:    d.CreatedAt,
	}
}

// ensure gorm is used (for potential future preloads)
var _ = gorm.ErrRecordNotFound

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
