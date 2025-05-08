package helper

import (
	"log"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

type OfferDetails struct {
	DiscountedPrice   float64
	OriginalPrice     float64
	DiscountPercentage float64
	OfferName         string
	IsOfferApplied    bool
}

// GetBestOfferForProduct calculates the best offer for a product based on its base price plus the variant's extra price
func GetBestOfferForProduct(product *adminModels.Product, variantExtraPrice float64) OfferDetails {
	var productOffer adminModels.ProductOffer
	var categoryOffer adminModels.CategoryOffer
	var result OfferDetails

	// Calculate total price (base price + variant extra price)
	totalPrice := product.Price + variantExtraPrice
	result.OriginalPrice = totalPrice
	result.DiscountedPrice = totalPrice
	result.IsOfferApplied = false

	currentTime := time.Now()

	// Check for active product offer
	err := database.DB.Where("product_id = ? AND is_active = ? AND start_date <= ? AND end_date >= ?",
		product.ID, true, currentTime, currentTime).First(&productOffer).Error
	productDiscount := 0.0
	if err == nil {
		productDiscount = productOffer.Discount
		log.Printf("Found product offer for product_id=%d: %s, discount=%.2f%%", product.ID, productOffer.OfferName, productDiscount)
	} else if err == gorm.ErrRecordNotFound {
		log.Printf("No active product offer found for product_id=%d", product.ID)
	} else {
		log.Printf("Error fetching product offer for product_id=%d: %v", product.ID, err)
	}

	// Check for active category offer
	var category adminModels.Category
	err = database.DB.Where("category_name = ?", product.CategoryName).First(&category).Error
	if err == nil {
		log.Printf("Found category for product_id=%d: category_name=%s, category_id=%d", product.ID, product.CategoryName, category.ID)
		err = database.DB.Where("category_id = ? AND is_active = ? AND start_date <= ? AND end_date >= ?",
			category.ID, true, currentTime, currentTime).First(&categoryOffer).Error
		if err == nil {
			log.Printf("Found category offer for category_id=%d: %s, discount=%.2f%%", category.ID, categoryOffer.OfferName, categoryOffer.Discount)
			if categoryOffer.Discount > productDiscount {
				result.DiscountPercentage = categoryOffer.Discount
				result.OfferName = categoryOffer.OfferName
				result.IsOfferApplied = true
			} else if productDiscount > 0 {
				result.DiscountPercentage = productOffer.Discount
				result.OfferName = productOffer.OfferName
				result.IsOfferApplied = true
			}
		} else if err == gorm.ErrRecordNotFound {
			log.Printf("No active category offer found for category_id=%d", category.ID)
			if productDiscount > 0 {
				result.DiscountPercentage = productOffer.Discount
				result.OfferName = productOffer.OfferName
				result.IsOfferApplied = true
			}
		} else {
			log.Printf("Error fetching category offer for category_id=%d: %v", category.ID, err)
		}
	} else if err == gorm.ErrRecordNotFound {
		log.Printf("Category not found for category_name=%s", product.CategoryName)
		if productDiscount > 0 {
			result.DiscountPercentage = productOffer.Discount
			result.OfferName = productOffer.OfferName
			result.IsOfferApplied = true
		}
	} else {
		log.Printf("Error fetching category for category_name=%s: %v", product.CategoryName, err)
	}

	// Apply the discount to the total price if an offer is found
	if result.IsOfferApplied {
		result.DiscountedPrice = totalPrice * (1 - result.DiscountPercentage/100)
		result.OriginalPrice = totalPrice
		log.Printf("Applied offer for product_id=%d: %s, original_price=%.2f, discounted_price=%.2f, discount=%.2f%%",
			product.ID, result.OfferName, result.OriginalPrice, result.DiscountedPrice, result.DiscountPercentage)
	}

	return result
}