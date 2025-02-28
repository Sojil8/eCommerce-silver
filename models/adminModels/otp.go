package adminModels

import (
	"time"

	"gorm.io/gorm"
)

type Otp struct{
	gorm.Model
	Email string `json:"email"`
	OTP string `json:"otp"`
	ExpTime time.Time
}