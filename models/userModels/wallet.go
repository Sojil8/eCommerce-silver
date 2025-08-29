package userModels

import (
	"gorm.io/gorm"
)

type Wallet struct {
	gorm.Model
	UserID  uint    `json:"user_id" gorm:"uniqueIndex"`
	Balance float64 `json:"balance" gorm:"default:0"`
}

type WalletTransaction struct {
	gorm.Model
	UserID        uint
	WalletID      uint
	Amount        float64
	LastBalance   float64
	Description   string
	Type          string
	Receipt       string
	OrderID       string
	TransactionID string
	PaymentMethod string
}
