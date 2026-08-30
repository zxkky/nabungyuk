package models

import (
	"time"
)

// ReminderFrequency represents the frequency of a reminder
type ReminderFrequency string

const (
	ReminderFrequencyDaily   ReminderFrequency = "daily"
	ReminderFrequencyWeekly  ReminderFrequency = "weekly"
	ReminderFrequencyMonthly ReminderFrequency = "monthly"
)

// Reminder represents a savings reminder
type Reminder struct {
	ID           uint              `json:"id" gorm:"primarykey;autoIncrement"`
	UserID       uint              `json:"user_id" gorm:"not null;index"`
	SavingGoalID *uint             `json:"saving_goal_id" gorm:"index"` // nullable
	Frequency    ReminderFrequency `json:"frequency" gorm:"type:varchar(20);not null"`
	Day          string            `json:"day" gorm:"type:varchar(20)"`           // weekly: day name, monthly: day number
	Time         string            `json:"time" gorm:"type:varchar(10);not null"` // HH:MM format
	IsActive     bool              `json:"is_active" gorm:"default:true"`
	LastSentAt   *time.Time        `json:"last_sent_at"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`

	// Relationships
	User       User        `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	SavingGoal *SavingGoal `json:"saving_goal,omitempty" gorm:"foreignKey:SavingGoalID;constraint:OnDelete:CASCADE"`
}

// IsValidFrequency checks if the frequency is valid
func (r *Reminder) IsValidFrequency() bool {
	switch r.Frequency {
	case ReminderFrequencyDaily, ReminderFrequencyWeekly, ReminderFrequencyMonthly:
		return true
	default:
		return false
	}
}
