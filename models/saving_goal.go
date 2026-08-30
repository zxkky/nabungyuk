package models

import (
	"time"
)

// SavingGoal represents a savings target
type SavingGoal struct {
	ID           uint      `json:"id" gorm:"primarykey;autoIncrement"`
	UserID       uint      `json:"user_id" gorm:"not null;index"`
	Name         string    `json:"name" gorm:"type:varchar(100);not null"`
	TargetAmount int64     `json:"target_amount" gorm:"not null"` // stored as rupiah
	Deadline     time.Time `json:"deadline" gorm:"not null"`
	Icon         string    `json:"icon" gorm:"type:varchar(50)"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	User     User            `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Deposits []SavingDeposit `json:"deposits,omitempty" gorm:"foreignKey:SavingGoalID;constraint:OnDelete:CASCADE"`
}

// Icon constants for saving goals
const (
	IconDefault   = "default"
	IconLaptop    = "laptop"
	IconHouse     = "house"
	IconCar       = "car"
	IconVacation  = "vacation"
	IconShopping  = "shopping"
	IconEducation = "education"
	IconHealth    = "health"
)

// GetIcons returns list of available icons
func GetIcons() []string {
	return []string{
		IconDefault,
		IconLaptop,
		IconHouse,
		IconCar,
		IconVacation,
		IconShopping,
		IconEducation,
		IconHealth,
	}
}
