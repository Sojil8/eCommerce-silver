package adminModels

import (
	"time"
)

type PaymentDetails struct {
	ID                uint   `gorm:"primaryKey"`
	OrderID           uint   `gorm:"index"`
	UserID            uint   `gorm:"index"`
	RazorpayOrderID   string `gorm:"size:100"`
	RazorpayPaymentID string `gorm:"size:100"`
	RazorpaySignature string `gorm:"size:255"`
	PaymentMethod     string
	Amount            float64
	Status            string `gorm:"size:50"`
	Attempts          int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
