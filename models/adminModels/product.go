package adminModels

import "gorm.io/gorm"

type Product struct {
	*gorm.Model
	ProductName string `gorm:"not null" json:"productName"`
	//will complete soon
}
