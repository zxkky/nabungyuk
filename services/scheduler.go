package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/models"
)

// Scheduler is a background worker that checks reminders and sends emails
type Scheduler struct {
	email *EmailService
	stop  chan struct{}
}

// NewScheduler creates a scheduler with an email service
func NewScheduler(email *EmailService) *Scheduler {
	return &Scheduler{
		email: email,
		stop:  make(chan struct{}),
	}
}

// Start runs the scheduler loop every `interval`. Use a small interval for development/testing.
func (s *Scheduler) Start(interval time.Duration) {
	log.Printf("[scheduler] started, checking reminders every %s", interval)
	go func() {
		for {
			select {
			case <-s.stop:
				log.Println("[scheduler] stopped")
				return
			case <-time.After(interval):
				s.checkAndSend()
			}
		}
	}()
}

// Stop terminates the scheduler loop
func (s *Scheduler) Stop() {
	close(s.stop)
}

// checkAndSend finds due active reminders and sends emails
func (s *Scheduler) checkAndSend() {
	var reminders []models.Reminder
	if err := config.DB.Where("is_active = ?", true).Find(&reminders).Error; err != nil {
		log.Printf("[scheduler] failed to fetch reminders: %v", err)
		return
	}

	now := time.Now()
	nowTimeString := now.Format("15:04")

	for _, r := range reminders {
		// Check time is due (allow a small tolerance window of +/- 1 minute so
		// a scheduler that ticks at :30 still catches reminders at :00)
		if !isTimeDue(r.Time, nowTimeString) {
			continue
		}

		// Check frequency schedule
		if !isFrequencyDue(&r, now) {
			continue
		}

		// Check last_sent_at to avoid duplicates
		if lastSentRecently(&r, now) {
			continue
		}

		// Send email
		if err := s.sendForReminder(&r, now); err != nil {
			log.Printf("[scheduler] error sending reminder #%d: %v", r.ID, err)
			continue
		}

		// Update last_sent_at
		t := now
		r.LastSentAt = &t
		config.DB.Model(&r).Update("last_sent_at", t)
		log.Printf("[scheduler] reminder #%d sent to user #%d", r.ID, r.UserID)
	}
}

// isTimeDue checks if the current time matches (within +/- 1 minute) the reminder time
func isTimeDue(scheduledTime, nowTime string) bool {
	sMin, sOK := parseMinutes(scheduledTime)
	nMin, nOK := parseMinutes(nowTime)
	if !sOK || !nOK {
		return false
	}
	diff := nMin - sMin
	if diff < 0 {
		diff = -diff
	}
	// Within 1 minute (60s) of the scheduled time — good for short tick intervals.
	return diff <= 1
}

// parseMinutes converts "HH:MM" to minutes since midnight
func parseMinutes(hhmm string) (int, bool) {
	parts := strings.Split(hhmm, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// isFrequencyDue checks whether today matches the reminder's frequency & day
func isFrequencyDue(r *models.Reminder, now time.Time) bool {
	switch r.Frequency {
	case models.ReminderFrequencyDaily:
		return true

	case models.ReminderFrequencyWeekly:
		// Day holds a weekday name (e.g. "Monday", "Senin" normalized to English)
		day := normalizeWeekday(r.Day)
		weekday := strings.ToLower(now.Weekday().String())
		return weekday == day

	case models.ReminderFrequencyMonthly:
		// Day 29/30/31 falls back to the last calendar day when that day
		// does not exist in the current month.
		dayNum, err := strconv.Atoi(strings.TrimSpace(r.Day))
		if err != nil || dayNum < 1 || dayNum > 31 {
			return false
		}
		lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
		targetDay := dayNum
		if targetDay > lastDay {
			targetDay = lastDay
		}
		return now.Day() == targetDay

	default:
		return false
	}
}

// lastSentRecently returns true if a reminder was sent within the same day (prevents duplicates)
func lastSentRecently(r *models.Reminder, now time.Time) bool {
	if r.LastSentAt == nil {
		return false
	}
	return r.LastSentAt.Year() == now.Year() &&
		r.LastSentAt.Month() == now.Month() &&
		r.LastSentAt.Day() == now.Day()
}

// sendForReminder loads goal info and sends the reminder email
func (s *Scheduler) sendForReminder(r *models.Reminder, now time.Time) error {
	// Load user
	var user models.User
	if err := config.DB.First(&user, r.UserID).Error; err != nil {
		return err
	}

	// Load saving goal (optional)
	var (
		goalName          = "Tabungan"
		progress    int64 = 0
		suggested   int64 = 500000
		deadlineStr       = "-"
	)

	if r.SavingGoalID != nil {
		var goal models.SavingGoal
		if err := config.DB.First(&goal, *r.SavingGoalID).Error; err == nil {
			goalName = goal.Name
			current := sumDepositsForGoal(goal.ID)
			if goal.TargetAmount > 0 {
				progress = min(int64(float64(current)/float64(goal.TargetAmount)*100), 100)
			}
			// Suggested saving: remaining / months left (min 50k)
			remaining := max(goal.TargetAmount-current, 0)
			monthsLeft := int(now.AddDate(1, 0, 0).Sub(goal.Deadline).Hours() / (24 * 30))
			if goal.Deadline.After(now) {
				monthsLeft = int(goal.Deadline.Sub(now).Hours() / (24 * 30))
			}
			if monthsLeft < 1 {
				monthsLeft = 1
			}
			suggested = max(remaining/int64(monthsLeft), 50000)
			deadlineStr = goal.Deadline.Format("02 Jan 2006")
		}
	}

	if !s.email.IsConfigured() {
		return fmt.Errorf("SMTP belum dikonfigurasi")
	}

	return s.email.SendReminder(user.Email, user.Name, goalName, suggested, progress, deadlineStr)
}

// sumDepositsForGoal computes total deposits for a goal
func sumDepositsForGoal(goalID uint) int64 {
	var sum int64
	config.DB.Model(&models.SavingDeposit{}).
		Where("saving_goal_id = ?", goalID).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum)
	return sum
}

var _ = gorm.ErrRecordNotFound // ensure gorm import stays (future preloads)
func normalizeWeekday(day string) string {
	day = strings.ToLower(strings.TrimSpace(day))
	translations := map[string]string{
		"minggu": "sunday", "senin": "monday", "selasa": "tuesday",
		"rabu": "wednesday", "kamis": "thursday", "jumat": "friday", "jum'at": "friday", "sabtu": "saturday",
	}
	if v, ok := translations[day]; ok {
		return v
	}
	return day
}
