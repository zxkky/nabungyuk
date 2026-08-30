package models

import (
	"time"
)

// SavingDeposit represents a deposit into a saving goal
type SavingDeposit struct {
	ID           uint      `json:"id" gorm:"primarykey;autoIncrement"`
	SavingGoalID uint      `json:"saving_goal_id" gorm:"not null;index"`
	Amount       int64     `json:"amount" gorm:"not null"` // stored as rupiah
	Note         string    `json:"note" gorm:"type:text"`
	DepositDate  time.Time `json:"deposit_date" gorm:"not null;index"`
	CreatedAt    time.Time `json:"created_at"`

	// Relationship
	SavingGoal SavingGoal `json:"-" gorm:"foreignKey:SavingGoalID;constraint:OnDelete:CASCADE"`
}
