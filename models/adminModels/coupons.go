package adminModels

import (
	"time"

	"gorm.io/gorm"
)

type Coupons struct {
	gorm.Model
	CouponCode         string    `gorm:"unique" json:"couponcode"`
	DiscountPercentage float64   `json:"discount_percentage"`
	MinPurchaseAmount  float64   `json:"min_purchase_amount"`
	ExpiryDate         time.Time `json:"expirydate"`
	UsageLimit         int       `json:"usage_limit"`
	UsedCount          int       `gorm:"default:0" json:"usage_count"`
	IsActive           bool      `json:"is_active"`
}