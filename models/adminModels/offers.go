package adminModels

import (
	"time"

	"gorm.io/gorm"
)

type ProductOffer struct {
	gorm.Model
	ProductID uint      `json:"product_id"`
	OfferName string    `json:"offer_name"`
	Discount  float64   `json:"discount"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsActive  bool      `json:"is_active"`
}

type CategoryOffer struct {
	gorm.Model
	CategoryID uint      `json:"category_id"`
	OfferName  string    `json:"offer_name"`
	Discount   float64   `json:"discount"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	IsActive   bool      `json:"is_active"`
}
