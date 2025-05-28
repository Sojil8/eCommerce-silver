// Corrected ShippingAddress model
package adminModels

import "gorm.io/gorm"

type ShippingAddress struct {
	gorm.Model
	OrderID        string `json:"order_id" gorm:"column:order_id;not null"`
	AddressType    string `json:"address_type"`
	UserID         uint   `json:"user_id"`
	Name           string `json:"name"`
	City           string `json:"city"`
	Landmark       string `json:"landmark"`
	State          string `json:"state"`
	Pincode        string `json:"pincode"`
	Phone          string `json:"phone"`
	AlternatePhone string `json:"alternate_phone"`
	Instructions   string `json:"instructions"`
	TrackingStatus string `json:"tracking_status"`
}
