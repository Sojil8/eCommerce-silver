package userModels

import "gorm.io/gorm"

type Orders struct {
	*gorm.Model
	UserID      uint    `json:"user_id"`
	User        Users   `json:"user"`
	OrderStatus string  `json:"order_status"`
	ProductName string  `json:"product_name"`
	SubTotal    float64 `json:"sub_total"`
	Amount      float64 `json:"amount"`
	Status      string  `json:"status"`
}

// id int [pk]
//   user_id int
//   order_status varchar**
//   order_date timestamp
//   address_id int
//   coupon_id int
//   created_at timestamp
//   updated_at timestamp
//   total_amount decimal
