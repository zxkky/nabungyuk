package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID        uint      `json:"id" gorm:"primarykey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`
	Email     string    `json:"email" gorm:"type:varchar(100);unique;not null"`
	Password  string    `json:"-" gorm:"type:varchar(255);not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Transactions []Transaction `json:"transactions,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	SavingGoals  []SavingGoal  `json:"saving_goals,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Reminders    []Reminder    `json:"reminders,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// PublicUser returns user data without sensitive fields
func (u *User) PublicUser() map[string]interface{} {
	return map[string]interface{}{
		"id":         u.ID,
		"name":       u.Name,
		"email":      u.Email,
		"created_at": u.CreatedAt,
	}
}
