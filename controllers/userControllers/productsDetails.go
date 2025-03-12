package controllers

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetProductDetails(c *gin.Context) {
	productID := c.Param("id")
	var product adminModels.Product

	// Fetch product with variants and join with categories
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.id = ? AND products.is_listed = ? AND categories.status = ?", productID, true, true).
		Preload("Variants").First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.Redirect(http.StatusSeeOther, "/home")
			return
		}
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Error fetching product", "Database error", "")
		return
	}

	// Check stock in any variant
	hasStock := false
	for _, variant := range product.Variants {
		if variant.Stock > 0 {
			hasStock = true
			break
		}
	}
	if !hasStock {
		c.Redirect(http.StatusSeeOther, "/home")
		return
	}

	// Calculate discount
	discountPercentage := 0
	if product.OriginalPrice > product.Price && product.OriginalPrice > 0 {
		discountPercentage = int(((product.OriginalPrice - product.Price) / product.OriginalPrice) * 100)
	}

	// Fetch related products with stock and listed categories
	var relatedProducts []adminModels.Product
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.category_name = ? AND products.id != ? AND products.is_listed = ? AND categories.status = ?",
			product.CategoryName, product.ID, true, true).
		Preload("Variants").Limit(4).Find(&relatedProducts).Error; err != nil {
		// Log error silently; don’t fail the request
	}

	availableRelatedProducts := []adminModels.Product{}
	for _, rp := range relatedProducts {
		for _, v := range rp.Variants {
			if v.Stock > 0 {
				availableRelatedProducts = append(availableRelatedProducts, rp)
				break
			}
		}
	}

	c.HTML(http.StatusOK, "productDetails.html", gin.H{
		"Product":            product,
		"OriginalPrice":      product.OriginalPrice,
		"DiscountPercentage": discountPercentage,
		"RelatedProducts":    availableRelatedProducts,
		"Category":           product.CategoryName,
	})
}
