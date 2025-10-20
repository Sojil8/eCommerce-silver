package userModels

import "gorm.io/gorm"

type Users struct {
	gorm.Model
	UserName      string    `gorm:"column:user_name;not null" json:"user_name"`
	Email         string    `gorm:"column:email;unique;not null" json:"email"`
	Password      string    `gorm:"column:password;not null" json:"password"`
	Phone         string    `gorm:"column:phone;not null" json:"phone"`
	FirstName     string    `gorm:"column:first_name" json:"first_name"`
	LastName      string    `gorm:"column:last_name" json:"last_name"`
	ProfileImage  string    `gorm:"column:profile_image" json:"profile_image"`
	IsBlocked     bool      `gorm:"column:is_blocked;default:false" json:"is_blocked"`
	// Addresses     []Address `gorm:"foreignKey:UserID" json:"addresses"`
	ReferralToken string    `gorm:"column:referral_token" json:"referral_token"`
}
