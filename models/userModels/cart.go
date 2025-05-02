package userModels

import (
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

type Cart struct {
    gorm.Model
    UserID     uint   `json:"user_id"`
    TotalPrice float64   `json:"total"`
	CouponID  uint `json:"coupon_id"`
    CartItems  []CartItem `gorm:"foreignKey:CartID"` 
}

type CartItem struct {
	gorm.Model
	CartID     uint                 `json:"cart_id"`
	Cart       Cart                 `gorm:"foreignKey:CartID"`
	ProductID  uint                 `json:"product_id"`
	Product    adminModels.Product  `gorm:"foreignKey:ProductID"`
	VariantsID uint                 `json:"variants_id"`
	Variants   adminModels.Variants `gorm:"foreignKey:VariantsID"`
	Quantity   uint                 `json:"quantity"`
	Price      float64              `json:"price"`
}
