package helper

import (
	"log"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/robfig/cron/v3"
)

// UpdateExpiredCoupons checks all coupons and sets IsActive to false for expired ones
func UpdateExpiredCoupons() {
	var coupons []adminModels.Coupons
	if err := database.DB.Where("is_active = ? AND expiry_date < ?", true, time.Now()).Find(&coupons).Error; err != nil {
		log.Printf("Error fetching coupons: %v", err)
		return
	}

	for _, coupon := range coupons {
		coupon.IsActive = false
		if err := database.DB.Save(&coupon).Error; err != nil {
			log.Printf("Error updating coupon %s: %v", coupon.CouponCode, err)
		} else {
			log.Printf("Coupon %s deactivated due to expiry", coupon.CouponCode)
		}
	}
}

// StartCouponExpiryScheduler initializes the cron job for checking expired coupons
func StartCouponExpiryScheduler() {
	c := cron.New()

	// Schedule the task to run every day at midnight
	_, err := c.AddFunc("0 0 * * *", UpdateExpiredCoupons)
	if err != nil {
		log.Fatalf("Error scheduling coupon expiry task: %v", err)
	}

	// Start the scheduler
	c.Start()
	log.Println("Coupon expiry scheduler started")
}