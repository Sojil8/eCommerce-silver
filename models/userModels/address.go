package userModels

import "gorm.io/gorm"

type Address struct {
	gorm.Model
	UserID         uint   `json:"user_id"`
	AddressType    string `json:"address_type"`
	Name           string `json:"name"`
	City           string `json:"city"`
	Landmark       string `json:"landmark"`
	State          string `json:"state"`
	Pincode        string `json:"pincode"`
	Phone          string `json:"phone"`
	AlternatePhone string `json:"alternate_phone"`
	IsDefault      bool   `gorm:"default:false" json:"is_default"`
}
