package userModels

import (
	"time"

	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

type Orders struct {
	gorm.Model
	UserID        uint        `json:"user_id"`
	User          Users       `gorm:"foreignKey:UserID" json:"user"`
	OrderIdUnique string      `gorm:"type:varchar(255);unique" json:"order_id"`
	AddressID     uint        `json:"address_id"`
	TotalPrice    float64     `json:"total_price"`
	PaymentMethod string      `json:"payment_method"`
	OrderItems    []OrderItem `gorm:"foreignKey:OrderID" json:"order_items"`
	CouponID      uint        `json:"coupon_id"`
	Status        string      `json:"status"`
	Discount      float64	 `json:"discount"`
	Subtotal      float64   `json:"subtotal"`
	OrderDate     time.Time   `json:"order_date"`
}

type OrderItem struct {
	*gorm.Model
	OrderID    uint                 `json:"order_id"`
	ProductID  uint                 `json:"product_id" gorm:"index"`
	VariantsID uint                 `json:"variants_id" gorm:"index"`
	Quantity   uint                 `json:"quantity"`
	Price      float64              `json:"price"`
	Status     string               `json:"status"`
	Product    adminModels.Product  `gorm:"foreignKey:ProductID" json:"product"`
	Variants   adminModels.Variants `gorm:"foreignKey:VariantsID" json:"variants"`
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
	Status string `json:"status"`
}
