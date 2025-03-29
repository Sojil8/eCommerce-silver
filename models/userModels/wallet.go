package userModels

import "gorm.io/gorm"

type Wallet struct {
	gorm.Model
	UserID  uint    `json:"user_id" gorm:"uniqueIndex"`
	Balance float64 `json:"balance" gorm:"default:0"`
}
