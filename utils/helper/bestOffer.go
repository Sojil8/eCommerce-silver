package helper

import (
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OfferDetails struct {
	DiscountedPrice    float64
	OriginalPrice      float64
	DiscountPercentage float64
	OfferName          string
	IsOfferApplied     bool
	EndTime            time.Time
}

func GetBestOfferForProduct(product *adminModels.Product, variantExtraPrice float64) OfferDetails {
	pkg.Log.Debug("Calculating best offer for product",
		zap.Uint("productID", product.ID),
		zap.Float64("variantExtraPrice", variantExtraPrice))

	var productOffer adminModels.ProductOffer
	var categoryOffer adminModels.CategoryOffer
	var result OfferDetails

	totalPrice := product.Price + variantExtraPrice
	result.OriginalPrice = totalPrice
	result.DiscountedPrice = totalPrice
	result.IsOfferApplied = false

	currentTime := time.Now()

	// Check for product-specific offer
	err := database.DB.Where("product_id = ? AND is_active = ? AND start_date <= ? AND end_date >= ?",
		product.ID, true, currentTime, currentTime).First(&productOffer).Error
	productDiscount := 0.0
	switch err {
	case nil:
		productDiscount = productOffer.Discount
		pkg.Log.Info("Found product offer",
			zap.Uint("productID", product.ID),
			zap.String("offerName", productOffer.OfferName),
			zap.Float64("discount", productDiscount))
	case gorm.ErrRecordNotFound:
		pkg.Log.Debug("No active product offer found",
			zap.Uint("productID", product.ID))
	default:
		pkg.Log.Error("Error fetching product offer",
			zap.Uint("productID", product.ID),
			zap.Error(err))
	}

	// Check for category offer
	var category adminModels.Category
	err = database.DB.Where("category_name = ?", product.CategoryName).First(&category).Error
	switch err {
	case nil:
		pkg.Log.Debug("Found category",
			zap.Uint("productID", product.ID),
			zap.String("categoryName", product.CategoryName),
			zap.Uint("categoryID", category.ID))
		err = database.DB.Where("category_id = ? AND is_active = ? AND start_date <= ? AND end_date >= ?",
			category.ID, true, currentTime, currentTime).First(&categoryOffer).Error
		switch err {
		case nil:
			pkg.Log.Info("Found category offer",
				zap.Uint("categoryID", category.ID),
				zap.String("offerName", categoryOffer.OfferName),
				zap.Float64("discount", categoryOffer.Discount))
			if categoryOffer.Discount > productDiscount {
				result.DiscountPercentage = categoryOffer.Discount
				result.OfferName = categoryOffer.OfferName
				result.IsOfferApplied = true
				result.EndTime = categoryOffer.EndDate
			} else if productDiscount > 0 {
				result.DiscountPercentage = productOffer.Discount
				result.OfferName = productOffer.OfferName
				result.IsOfferApplied = true
				result.EndTime = productOffer.EndDate
			}
		case gorm.ErrRecordNotFound:
			pkg.Log.Debug("No active category offer found",
				zap.Uint("categoryID", category.ID))
			if productDiscount > 0 {
				result.DiscountPercentage = productOffer.Discount
				result.OfferName = productOffer.OfferName
				result.IsOfferApplied = true
				result.EndTime = productOffer.EndDate
			}
		default:
			pkg.Log.Error("Error fetching category offer",
				zap.Uint("categoryID", category.ID),
				zap.Error(err))
		}
	case gorm.ErrRecordNotFound:
		pkg.Log.Warn("Category not found",
			zap.String("categoryName", product.CategoryName))
		if productDiscount > 0 {
			result.DiscountPercentage = productOffer.Discount
			result.OfferName = productOffer.OfferName
			result.IsOfferApplied = true
			result.EndTime = productOffer.EndDate
		}
	default:
		pkg.Log.Error("Error fetching category",
			zap.String("categoryName", product.CategoryName),
			zap.Error(err))
	}

	if result.IsOfferApplied {
		result.DiscountedPrice = totalPrice * (1 - result.DiscountPercentage/100)
		pkg.Log.Info("Applied best offer for product",
			zap.Uint("productID", product.ID),
			zap.String("offerName", result.OfferName),
			zap.Float64("originalPrice", result.OriginalPrice),
			zap.Float64("discountedPrice", result.DiscountedPrice),
			zap.Float64("discountPercentage", result.DiscountPercentage),
			zap.Time("endTime", result.EndTime))
	} else {
		pkg.Log.Debug("No offer applied for product",
			zap.Uint("productID", product.ID),
			zap.Float64("originalPrice", result.OriginalPrice))
	}

	return result
}