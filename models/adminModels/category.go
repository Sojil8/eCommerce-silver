package adminModels

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	CategoryName string `gorm:"not null" json:"category_name"` // Changed to match frontend
	Description  string `gorm:"not null" json:"description"`
	Status       bool   `gorm:"not null" json:"status"`
}
