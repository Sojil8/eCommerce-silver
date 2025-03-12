package adminModels

import "gorm.io/gorm"

type Variants struct {
	gorm.Model
	ProductID  uint    `gorm:"not null" json:"product_id"`
	Color      string  `gorm:"not null" json:"color"`      
	ExtraPrice float64 `json:"extra_price"`                 
	Stock      uint    `gorm:"not null" json:"stock"`     

}
