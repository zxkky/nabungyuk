package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/models"
)

// ReminderController handles reminder-related requests
type ReminderController struct{}

// NewReminderController creates a new ReminderController
func NewReminderController() *ReminderController {
	return &ReminderController{}
}

// reminderResponse is the JSON response shape for a reminder
type reminderResponse struct {
	ID           uint       `json:"id"`
	UserID       uint       `json:"user_id"`
	SavingGoalID *uint      `json:"saving_goal_id"`
	GoalName     string     `json:"goal_name"`
	Frequency    string     `json:"frequency"`
	Day          string     `json:"day"`
	Time         string     `json:"time"`
	IsActive     bool       `json:"is_active"`
	LastSentAt   *time.Time `json:"last_sent_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// GetAllReminders returns all reminders for the authenticated user
func (rc *ReminderController) GetAllReminders(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	var reminders []models.Reminder
	if err := config.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&reminders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengambil reminder"})
		return
	}

	responses := make([]reminderResponse, len(reminders))
	for i, r := range reminders {
		responses[i] = toReminderResponse(r)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reminder berhasil diambil",
		"data": gin.H{
			"reminders": responses,
		},
	})
}

// GetReminder returns a specific reminder (only if owned by user)
func (rc *ReminderController) GetReminder(c *gin.Context) {
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

	var reminder models.Reminder
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&reminder).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Reminder tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reminder berhasil diambil",
		"data":    toReminderResponse(reminder),
	})
}

// CreateReminder creates a new reminder
func (rc *ReminderController) CreateReminder(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	var req struct {
		SavingGoalID *uint  `json:"saving_goal_id"`
		Frequency    string `json:"frequency" binding:"required"`
		Day          string `json:"day"`
		Time         string `json:"time" binding:"required"`
		IsActive     *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Validasi gagal: frequency dan time wajib diisi"})
		return
	}

	req.Frequency = strings.ToLower(strings.TrimSpace(req.Frequency))
	if !isValidFrequency(req.Frequency) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Frequency harus daily, weekly, atau monthly"})
		return
	}

	// Validate time format HH:MM
	if !isValidTimeFormat(req.Time) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Waktu tidak valid, gunakan format HH:MM (24 jam)"})
		return
	}

	// Normalize localized weekday names so FE may send Indonesian or English.
	req.Day = normalizeReminderDay(req.Frequency, req.Day)

	// Validate day based on frequency
	if err := validateDay(req.Frequency, req.Day); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Validate saving goal ownership if provided
	if req.SavingGoalID != nil {
		var goal models.SavingGoal
		if err := config.DB.Where("id = ? AND user_id = ?", *req.SavingGoalID, userID).First(&goal).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
			return
		}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	reminder := models.Reminder{
		UserID:       userID,
		SavingGoalID: req.SavingGoalID,
		Frequency:    models.ReminderFrequency(req.Frequency),
		Day:          strings.TrimSpace(req.Day),
		Time:         req.Time,
		IsActive:     isActive,
	}
	if err := config.DB.Create(&reminder).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal membuat reminder: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Reminder berhasil dibuat",
		"data":    toReminderResponse(reminder),
	})
}

// UpdateReminder updates a reminder (only if owned by user)
func (rc *ReminderController) UpdateReminder(c *gin.Context) {
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

	var reminder models.Reminder
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&reminder).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Reminder tidak ditemukan"})
		return
	}

	var body struct {
		SavingGoalID *uint   `json:"saving_goal_id"`
		Frequency    *string `json:"frequency"`
		Day          *string `json:"day"`
		Time         *string `json:"time"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Body JSON tidak valid"})
		return
	}

	hasUpdates := false

	if body.Frequency != nil {
		f := strings.ToLower(strings.TrimSpace(*body.Frequency))
		if !isValidFrequency(f) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Frequency harus daily, weekly, atau monthly"})
			return
		}
		reminder.Frequency = models.ReminderFrequency(f)
		hasUpdates = true
	}
	if body.Time != nil {
		if !isValidTimeFormat(*body.Time) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Waktu tidak valid, gunakan format HH:MM"})
			return
		}
		reminder.Time = *body.Time
		hasUpdates = true
	}
	if body.Day != nil {
		day := normalizeReminderDay(string(reminder.Frequency), *body.Day)
		if err := validateDay(string(reminder.Frequency), day); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		reminder.Day = day
		hasUpdates = true
	} else if body.Frequency != nil {
		// A frequency change must never leave an incompatible day value.
		day := normalizeReminderDay(string(reminder.Frequency), reminder.Day)
		if err := validateDay(string(reminder.Frequency), day); err != nil {
			reminder.Day = ""
		} else {
			reminder.Day = day
		}
	}
	if body.SavingGoalID != nil {
		var goal models.SavingGoal
		if err := config.DB.Where("id = ? AND user_id = ?", *body.SavingGoalID, userID).First(&goal).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Target tabungan tidak ditemukan"})
			return
		}
		reminder.SavingGoalID = body.SavingGoalID
		hasUpdates = true
	}
	if body.IsActive != nil {
		reminder.IsActive = *body.IsActive
		hasUpdates = true
	}

	// Changing the schedule creates a new delivery opportunity.
	if body.Frequency != nil || body.Day != nil || body.Time != nil {
		reminder.LastSentAt = nil
	}

	if !hasUpdates {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tidak ada field yang diupdate"})
		return
	}

	if err := config.DB.Save(&reminder).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengupdate reminder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reminder berhasil diupdate",
		"data":    toReminderResponse(reminder),
	})
}

// DeleteReminder deletes a reminder (only if owned by user)
func (rc *ReminderController) DeleteReminder(c *gin.Context) {
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

	var reminder models.Reminder
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&reminder).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Reminder tidak ditemukan"})
		return
	}

	if err := config.DB.Delete(&reminder).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal menghapus reminder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reminder berhasil dihapus",
	})
}

// --- Helpers ---

// isValidFrequency checks reminder frequency value
func isValidFrequency(f string) bool {
	switch f {
	case string(models.ReminderFrequencyDaily),
		string(models.ReminderFrequencyWeekly),
		string(models.ReminderFrequencyMonthly):
		return true
	}
	return false
}

// isValidTimeFormat checks "HH:MM" format (24-hour)
func isValidTimeFormat(t string) bool {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	return err1 == nil && err2 == nil && h >= 0 && h <= 23 && m >= 0 && m <= 59
}

// validateDay checks the day field matches the frequency
func validateDay(frequency, day string) error {
	switch frequency {
	case string(models.ReminderFrequencyDaily):
		return nil // day not needed
	case string(models.ReminderFrequencyWeekly):
		day = normalizeReminderDay(frequency, day)
		valid := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
		for _, v := range valid {
			if strings.EqualFold(day, v) {
				return nil
			}
		}
		return errNewMsg("Day untuk weekly harus nama hari (Sunday-Saturday) dalam bahasa Inggris")
	case string(models.ReminderFrequencyMonthly):
		d, err := strconv.Atoi(strings.TrimSpace(day))
		if err != nil || d < 1 || d > 31 {
			return errNewMsg("Day untuk monthly harus angka 1-31")
		}
		return nil
	}
	return errNewMsg("Frequency tidak valid")
}

// errNewMsg creates a simple error from a message
func errNewMsg(msg string) error {
	return errorString(msg)
}

// errorString is a simple error type
type errorString string

func (e errorString) Error() string { return string(e) }

// toReminderResponse builds the response shape, fetching goal name if present
func toReminderResponse(r models.Reminder) reminderResponse {
	goalName := ""
	if r.SavingGoalID != nil {
		var goal models.SavingGoal
		if err := config.DB.First(&goal, *r.SavingGoalID).Error; err == nil {
			goalName = goal.Name
		}
	}
	return reminderResponse{
		ID:           r.ID,
		UserID:       r.UserID,
		SavingGoalID: r.SavingGoalID,
		GoalName:     goalName,
		Frequency:    string(r.Frequency),
		Day:          r.Day,
		Time:         r.Time,
		IsActive:     r.IsActive,
		LastSentAt:   r.LastSentAt,
		CreatedAt:    r.CreatedAt,
	}
}
func normalizeReminderDay(frequency, day string) string {
	if frequency == string(models.ReminderFrequencyDaily) {
		return ""
	}
	day = strings.ToLower(strings.TrimSpace(day))
	if frequency == string(models.ReminderFrequencyMonthly) {
		return day
	}
	translations := map[string]string{
		"minggu": "sunday", "senin": "monday", "selasa": "tuesday",
		"rabu": "wednesday", "kamis": "thursday", "jumat": "friday", "jum'at": "friday", "sabtu": "saturday",
	}
	if english, ok := translations[day]; ok {
		return english
	}
	return day
}
