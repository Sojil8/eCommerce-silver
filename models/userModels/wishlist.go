package userModels

import (
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

type Wishlist struct {
    gorm.Model
    UserID    uint                `json:"user_id"`
    ProductID uint                `json:"product_id"`
    User      Users               `gorm:"foreignKey:UserID"`
    Product   adminModels.Product `gorm:"foreignKey:ProductID"`
}