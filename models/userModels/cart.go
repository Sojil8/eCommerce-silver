package userModels

import (
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

type Cart struct {
	gorm.Model
	UserID     uint       `json:"user_id"`
	User       Users      `gorm:"foreignKey:UserID"`
	TotalPrice float64    `json:"total"`
	CouponID   uint       `json:"coupon_id"`
	CartItems  []CartItem `gorm:"foreignKey:CartID"`
}

type CartItem struct {
	gorm.Model
	CartID             uint                 `json:"cart_id"`
	Cart               Cart                 `gorm:"foreignKey:CartID"`
	ProductID          uint                 `json:"product_id"`
	Product            adminModels.Product  `gorm:"foreignKey:ProductID"`
	VariantsID         uint                 `json:"variants_id"`
	Variants           adminModels.Variants `gorm:"foreignKey:VariantsID"`
	Quantity           uint                 `json:"quantity"`
	Price              float64              `json:"price"`               // Base price (without offer)
	DiscountedPrice    float64              `json:"discounted_price"`    // Price after offer
	OriginalPrice      float64              `json:"original_price"`      // Original price before offer
	DiscountPercentage float64              `json:"discount_percentage"` // Discount percentage
	OfferName          string               `json:"offer_name"`          // Name of the applied offer
	IsOfferApplied     bool                 `json:"is_offer_applied"`    // Whether an offer is applied
	ItemTotal          float64              `json:"-"`                   // Total price for this item (calculated field)
}
