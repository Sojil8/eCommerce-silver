package controllers

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GetUserProducts(c *gin.Context) {
	pkg.Log.Info("Starting product retrieval process")

	// Fetch all listed products with their categories and variants
	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed = ? AND categories.status = ?", true, true).
		Preload("Variants")

	var products []adminModels.Product
	if err := db.Find(&products).Error; err != nil {
		pkg.Log.Error("Failed to fetch products",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error: failed to fetch products", "error: failed to fetch products", "")
		return
	}

	if len(products) == 0 {
		pkg.Log.Warn("No products retrieved")
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
		pkg.Log.Warn("Failed to fetch new arrivals",
			zap.Error(err))
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

	pkg.Log.Debug("Processed products",
		zap.Int("product_count", len(productWithOffers)))

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

	pkg.Log.Debug("Processed new arrivals",
		zap.Int("new_arrival_count", len(newArrivalsWithOffers)))

	// Retrieve user data
	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Info("Rendering home page for guest user",
			zap.Int("product_count", len(productWithOffers)),
			zap.Int("new_arrival_count", len(newArrivalsWithOffers)))
		c.HTML(http.StatusOK, "home.html", gin.H{
			"status":        "success",
			"Products":      productWithOffers,
			"NewArrivals":   newArrivalsWithOffers,
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
		pkg.Log.Warn("Failed to fetch wishlist count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Info("Rendering home page for authenticated user",
		zap.Uint("user_id", userData.ID),
		zap.String("user_name", userNameStr),
		zap.Int("product_count", len(productWithOffers)),
		zap.Int("new_arrival_count", len(newArrivalsWithOffers)),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount))

	c.HTML(http.StatusOK, "home.html", gin.H{
		"status":        "success",
		"Products":      productWithOffers,
		"NewArrivals":   newArrivalsWithOffers,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}
