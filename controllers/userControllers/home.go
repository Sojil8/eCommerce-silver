package controllers

import (
	"net/http"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

type productQuery struct {
	Search   string `form:"search"`
	Sort     string `form:"sort"`
	Category string `form:"category"`
}

type ProductWithOffer struct {
	Product            adminModels.Product
	OfferDetails       helper.OfferDetails
	DiscountPercentage int
}

func GetUserProducts(c *gin.Context) {
	var query productQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed = ? AND categories.status = ?", true, true)

	if query.Search != "" {
		searchTerm := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(products.product_name) LIKE ?", searchTerm)
	}

	var products []adminModels.Product
	if err := db.Preload("Variants").Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch products",
		})
		return
	}

	var productsWithOffers []ProductWithOffer
	for _, product := range products {
		// In GetProductDetails
		var variantExtraPrice float64
		if len(product.Variants) > 0 {
			// Assume the first variant with stock is selected by default
			for _, variant := range product.Variants {
				if variant.Stock > 0 {
					variantExtraPrice = variant.ExtraPrice
					break
				}
			}
		}
		offerDetails := helper.GetBestOfferForProduct(&product, variantExtraPrice)
		discountPercentage := int(offerDetails.DiscountPercentage)
		productsWithOffers = append(productsWithOffers, ProductWithOffer{
			Product:            product,
			OfferDetails:       offerDetails,
			DiscountPercentage: discountPercentage,
		})
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "home.html", gin.H{
			"status":        "success",
			"Products":      productsWithOffers,
			"UserName":      "Guest",
			"WishlistCount": 0,
			"CartCount":     0,
			"ProfileImage":  "",
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

	c.HTML(http.StatusOK, "home.html", gin.H{
		"status":        "success",
		"Products":      productsWithOffers,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}
