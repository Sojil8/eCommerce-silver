package adminModels

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	Category_name string `gorm:"not null" json:"category_name"`
	Description   string `gorm:"not null" json:"description"`
	Status        bool   `gorm:"not null"`
}
