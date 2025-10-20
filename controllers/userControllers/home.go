package controllers

import (
	"log"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
)

func GetUserProducts(c *gin.Context) {
	// Fetch all listed products with their categories and variants
	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed = ? AND categories.status = ?", true, true).
		Preload("Variants")

	var products []adminModels.Product
	if err := db.Find(&products).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error: failed to fetch products", "error: failed to fetch products", "")
		return
	}

	if products == nil {
		log.Println("products are not retrieving")
	}

	// Fetch 5 most recent products for New Arrivals
	var newArrivals []adminModels.Product
	if err := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed = ? AND categories.status = ?", true, true).
		Preload("Variants").
		Order("products.created_at DESC").
		Limit(5).
		Find(&newArrivals).Error; err != nil {
		log.Println("error: failed to fetch new arrivals", err)
		// Continue even if new arrivals fail, to avoid breaking the page
		newArrivals = []adminModels.Product{}
	}

	// Structure to include offer details
	type ProductWithOffer struct {
		adminModels.Product
		OfferPrice         float64
		OriginalPrice      float64
		DiscountPercentage float64
		IsOffer            bool
		OfferName          string
	}

	// Process all products
	var productWithOffers []ProductWithOffer
	for _, product := range products {
		variantExtraPrice := 0.0
		if len(product.Variants) > 0 {
			variantExtraPrice = product.Variants[0].ExtraPrice
		}

		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		productWithOffers = append(productWithOffers, ProductWithOffer{
			Product:            product,
			OfferPrice:         offer.DiscountedPrice,
			OriginalPrice:      offer.OriginalPrice,
			DiscountPercentage: offer.DiscountPercentage,
			IsOffer:            offer.IsOfferApplied,
			OfferName:          offer.OfferName,
		})
	}

	// Process new arrivals
	var newArrivalsWithOffers []ProductWithOffer
	for _, product := range newArrivals {
		variantExtraPrice := 0.0
		if len(product.Variants) > 0 {
			variantExtraPrice = product.Variants[0].ExtraPrice
		}

		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		newArrivalsWithOffers = append(newArrivalsWithOffers, ProductWithOffer{
			Product:            product,
			OfferPrice:         offer.DiscountedPrice,
			OriginalPrice:      offer.OriginalPrice,
			DiscountPercentage: offer.DiscountPercentage,
			IsOffer:            offer.IsOfferApplied,
			OfferName:          offer.OfferName,
		})
	}

	// Retrieve user data
	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "home.html", gin.H{
			"status":            "success",
			"Products":          productWithOffers,
			"NewArrivals":       newArrivalsWithOffers, // Add new arrivals to template
			"UserName":          "Guest",
			"WishlistCount":     0,
			"CartCount":         0,
			"ProfileImage":      "",
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
		"status":            "success",
		"Products":          productWithOffers,
		"NewArrivals":       newArrivalsWithOffers, // Add new arrivals to template
		"UserName":          userNameStr,
		"ProfileImage":      userData.ProfileImage,
		"WishlistCount":     wishlistCount,
		"CartCount":         cartCount,
	})
}