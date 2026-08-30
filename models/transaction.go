package models

import (
	"time"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "income"
	TransactionTypeExpense TransactionType = "expense"
)

// Transaction represents a financial transaction
type Transaction struct {
	ID              uint            `json:"id" gorm:"primarykey;autoIncrement"`
	UserID          uint            `json:"user_id" gorm:"not null;index"`
	Type            TransactionType `json:"type" gorm:"type:varchar(20);not null"`
	Title           string          `json:"title" gorm:"type:varchar(255);not null"`
	Category        string          `json:"category" gorm:"type:varchar(50);not null"`
	Amount          int64           `json:"amount" gorm:"not null"` // stored as rupiah
	Receipt         string          `json:"receipt" gorm:"type:varchar(255)"`
	Note            string          `json:"note" gorm:"type:text"`
	TransactionDate time.Time       `json:"transaction_date" gorm:"not null;index"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	// Relationship
	User User `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// IsValidType checks if the transaction type is valid
func (t *Transaction) IsValidType() bool {
	return t.Type == TransactionTypeIncome || t.Type == TransactionTypeExpense
}
