package userModels

import "gorm.io/gorm"

type User struct {
	*gorm.Model
	UserName   string `gorm:"notnull" json:"user_name"`
	Email      string `gorm:"unique" json:"email"`
	Password   string `gorm:"notnull" json:"password"`
	Phone      string `gorm:"notnull" json:"phone"`
	First_name string `json:"first_name"`
	Last_name  string `json:"last_name"`
	Is_blocked bool   `gorm:"type : bool; check:Is_blocked in (true,false); default:false" json:"is_blocked"`
}
