package adminModels

import "gorm.io/gorm"

type RazorpayPayment struct {
	gorm.Model
	OrderID         uint    `json:"order_id"`
	RazorpayOrderID string  `json:"razorpay_order_id"`
	PaymentID       string  `json:"payment_id"`
	Signature       string  `json:"signature"`
	Amount          float64 `json:"amount"`
	Status          string  `json:"status" gorm:"default:'Created'"`
	UserID          uint    `json:"user_id"`
}
