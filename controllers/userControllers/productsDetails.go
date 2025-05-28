package controllers

import (
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetProductDetails(c *gin.Context) {
	productID := c.Param("id")
	var product adminModels.Product

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

	var variant adminModels.Variants
	if err := database.DB.Where("product_id = ?", productID).First(&variant).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Variant Not found", "Variant Not Found", "")
		return
	}

	var hasStock bool
	for _, variant := range product.Variants {
		if variant.Stock > 0 {
			hasStock = true
			break
		}
	}

	type ProductWithOffer struct {
		adminModels.Product
		OfferPrice         float64
		OriginalPrice      float64
		DiscountPercentage float64
		IsOffer            bool
		OfferName          string
		OfferEndTime       time.Time
	}

	offer := helper.GetBestOfferForProduct(&product, variant.ExtraPrice)

	productWithOffers := ProductWithOffer{
		Product:            product,
		OfferPrice:         offer.DiscountedPrice,
		OriginalPrice:      offer.OriginalPrice,
		DiscountPercentage: offer.DiscountPercentage,
		IsOffer:            offer.IsOfferApplied,
		OfferName:          offer.OfferName,
		OfferEndTime:       offer.EndTime,
	}

	var relatedProducts []adminModels.Product
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.category_name = ? AND products.id != ? AND products.is_listed = ? AND categories.status = ?",
			product.CategoryName, product.ID, true, true).
		Preload("Variants").Limit(4).Find(&relatedProducts).Error; err != nil {
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

	breadcrumbs := config.GenerateBreadcrumbs(
		config.Breadcrumb{Name: "Shop", URL: "/shop"},
		config.Breadcrumb{Name: product.ProductName, URL: ""},
	)

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "productDetails.html", gin.H{
			"Product":         productWithOffers,
			"RelatedProducts": availableRelatedProducts,
			"Category":        product.CategoryName,
			"HasStock":        hasStock,
			"IsInStock":       hasStock,
			"Breadcrumbs":     breadcrumbs,
			"status":          "success",
			"UserName":        "Guest",
			"WishlistCount":   0,
			"CartCount":       0,
			"ProfileImage":    "",
		})
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		cartCount = 0
	}

	c.HTML(http.StatusOK, "productDetails.html", gin.H{
		"Product":         productWithOffers,
		"RelatedProducts": availableRelatedProducts,
		"Category":        product.CategoryName,
		"Breadcrumbs":     breadcrumbs,
		"HasStock":        hasStock,
		"IsInStock":       hasStock,
		"UserName":        userNameStr,
		"ProfileImage":    userData.ProfileImage,
		"WishlistCount":   wishlistCount,
		"CartCount":       cartCount,
	})
}