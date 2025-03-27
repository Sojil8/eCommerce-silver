package userModels

import "gorm.io/gorm"

type Orders struct {
	*gorm.Model
	UserID      uint    `json:"user_id"`
	AddressID   uint    `json:"address_id"`
	TotalPrice  float64 `json:"total_price"`
	Status      string  `json:"status"`
	PaymentMethod string  `json:"payment_method"`
	OrderItems []OrderItem 	`gorm:"foreignKey:OrderID" json:"order_items"`

}

type OrderItem struct {
	*gorm.Model
	OrderID uint `json:"order_id"`
	ProductID uint `json:"product_id"`
	VariantsID uint `json:"variants_id"`
	Quantity uint 	`json:"quantity"`
	Price float64 	`json:"price"`
}
