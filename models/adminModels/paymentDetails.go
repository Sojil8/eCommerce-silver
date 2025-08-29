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
	AddressID         uint `gorm:"index"`
	Amount            float64
	Status            string `gorm:"size:50"`
	FailureReason     string `gorm:"type:text" json:"failure_reason,omitempty"` 
	Attempts      int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TransactionID string
}
