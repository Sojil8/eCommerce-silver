package userModels

import (
	"time"

	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

type Orders struct {
	gorm.Model
	UserID          uint                        `json:"user_id"`
	User            Users                       `gorm:"foreignKey:UserID" json:"user"`
	OrderIdUnique   string                      `gorm:"type:varchar(255);unique" json:"order_id"`
	AddressID       uint                        `json:"address_id"`
	ShippingAddress adminModels.ShippingAddress `gorm:"foreignKey:AddressID" json:"shipping_address"`
	TotalPrice      float64                     `json:"total_price"`
	ShippingCost    float64                     `json:"shipping_cost"`
	PaymentMethod   string                      `json:"payment_method"`
	PaymentStatus   string                      `json:"payment_status"`
	OrderItems      []OrderItem                 `gorm:"foreignKey:OrderID" json:"order_items"`
	CouponID        uint                        `json:"coupon_id"`
	CouponCode      string                      `json:"coupon_code"`
	CouponDiscount  float64                     `json:"coupon_discount"`
	OfferDiscount   float64                     `json:"offer_discount"`
	TotalDiscount   float64                     `json:"total_discount"`
	Status          string                      `json:"status"`
	Subtotal        float64                     `json:"subtotal"`
	OrderDate       time.Time                   `json:"order_date"`

	CancellationStatus string `json:"cancellation_status"`
}

type OrderItem struct {
	gorm.Model
	OrderID           uint                 `json:"order_id"`
	ProductID         uint                 `json:"product_id" gorm:"index"`
	VariantsID        uint                 `json:"variants_id" gorm:"index"`
	Quantity          uint                 `json:"quantity"`
	UnitPrice         float64              `json:"unit_price"`
	ItemTotal         float64              `json:"item_total"`
	DiscountAmount    float64              `json:"discount_amount"`
	OfferName         string               `json:"offer_name"`
	Status            string               `json:"status"`
	ReturnStatus      string               `json:"return_status"`
	VariantAttributes string               `json:"variant_attributes"`
	Product           adminModels.Product  `gorm:"foreignKey:ProductID" json:"product"`
	Variants          adminModels.Variants `gorm:"foreignKey:VariantsID" json:"variants"`
}

type Cancellation struct {
	gorm.Model
	OrderID uint   `json:"order_id"`
	ItemID  *uint  `json:"item_id,omitempty"`
	Reason  string `json:"reason"`
}

type Return struct {
	gorm.Model
	OrderID uint   `json:"order_id"`
	Order   Orders `gorm:"foreignKey:OrderID" json:"order"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
}
