package adminModels

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	CategoryName string `gorm:"not null" json:"category_name"`
	Description  string `gorm:"not null" json:"description"`
	Status       bool   `gorm:"not null" json:"status"`
	Offers       []CategoryOffer
	Product      []Product `gorm:"foreignKey:CategoryID"`
}
