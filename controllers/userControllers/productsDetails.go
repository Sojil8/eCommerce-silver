package controllers

import (
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type VariantWithOffer struct {
	adminModels.Variants
	OfferPrice         float64
	OriginalPrice      float64
	DiscountPercentage float64
	IsOffer            bool
	OfferName          string
	OfferEndTime       time.Time
}

type ProductWithOffer struct {
	adminModels.Product
	OfferPrice         float64
	OriginalPrice      float64
	DiscountPercentage float64
	IsOffer            bool
	OfferName          string
	OfferEndTime       time.Time
	Variants           []VariantWithOffer
	IsInWishlist       bool `json:"is_in_wishlist"` // Added wishlist status
}

func GetProductDetails(c *gin.Context) {
	pkg.Log.Info("Starting product details retrieval")

	productID := c.Param("id")

	pkg.Log.Debug("Processing product details request",
		zap.String("product_id", productID),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	var product adminModels.Product
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.id = ? AND products.is_listed = ? AND categories.status = ?", productID, true, true).
		Preload("Variants").First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Warn("Product not found or not listed",
				zap.String("product_id", productID))
			c.Redirect(http.StatusSeeOther, "/home")
			return
		}
		pkg.Log.Error("Error fetching product",
			zap.String("product_id", productID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Error fetching product", "Database error", "")
		return
	}

	pkg.Log.Debug("Fetched product",
		zap.String("product_id", productID),
		zap.String("product_name", product.ProductName),
		zap.String("category_name", product.CategoryName))

	var hasStock bool
	var variantsWithOffer []VariantWithOffer
	for _, variant := range product.Variants {
		if variant.Stock > 0 {
			hasStock = true
		}
		offer := helper.GetBestOfferForProduct(&product, variant.ExtraPrice)
		variantsWithOffer = append(variantsWithOffer, VariantWithOffer{
			Variants:           variant,
			OfferPrice:         offer.DiscountedPrice,
			OriginalPrice:      offer.OriginalPrice,
			DiscountPercentage: offer.DiscountPercentage,
			IsOffer:            offer.IsOfferApplied,
			OfferName:          offer.OfferName,
			OfferEndTime:       offer.EndTime,
		})
	}

	pkg.Log.Debug("Processed product variants",
		zap.String("product_id", productID),
		zap.Int("variant_count", len(variantsWithOffer)),
		zap.Bool("has_stock", hasStock))

	// Use the first variant's offer for the main product display
	var offer helper.OfferDetails
	if len(product.Variants) > 0 {
		offer = helper.GetBestOfferForProduct(&product, product.Variants[0].ExtraPrice)
	} else {
		offer = helper.GetBestOfferForProduct(&product, 0)
	}

	productWithOffers := ProductWithOffer{
		Product:            product,
		OfferPrice:         offer.DiscountedPrice,
		OriginalPrice:      offer.OriginalPrice,
		DiscountPercentage: offer.DiscountPercentage,
		IsOffer:            offer.IsOfferApplied,
		OfferName:          offer.OfferName,
		OfferEndTime:       offer.EndTime,
		Variants:           variantsWithOffer,
	}

	// Check if main product is in wishlist
	isInWishlist := false
	user, exists := c.Get("user")
	userID := uint(0)
	userNameStr := "Guest"
	if exists {
		userData := user.(userModels.Users)
		userID = userData.ID
		userName, _ := c.Get("user_name")
		userNameStr = userName.(string)
		var count int64
		if err := database.DB.Model(&userModels.Wishlist{}).
			Where("user_id = ? AND product_id = ?", userData.ID, product.ID).
			Count(&count).Error; err != nil {
			pkg.Log.Error("Error checking wishlist",
				zap.Uint("user_id", userData.ID),
				zap.String("product_id", productID),
				zap.Error(err))
		} else {
			isInWishlist = count > 0
			pkg.Log.Debug("Checked wishlist status",
				zap.Uint("user_id", userData.ID),
				zap.String("product_id", productID),
				zap.Bool("is_in_wishlist", isInWishlist))
		}
		productWithOffers.IsInWishlist = isInWishlist
	}

	pkg.Log.Info("Product with offer details prepared",
		zap.String("product_id", productID),
		zap.String("product_name", productWithOffers.ProductName),
		zap.Float64("offer_price", productWithOffers.OfferPrice),
		zap.Float64("original_price", productWithOffers.OriginalPrice),
		zap.Float64("discount_percentage", productWithOffers.DiscountPercentage),
		zap.Bool("is_offer", productWithOffers.IsOffer),
		zap.String("offer_name", productWithOffers.OfferName),
		zap.Time("offer_end_time", productWithOffers.OfferEndTime),
		zap.Bool("is_in_wishlist", isInWishlist),
		zap.Bool("has_stock", hasStock))

	var relatedProducts []adminModels.Product
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.category_name = ? AND products.id != ? AND products.is_listed = ? AND categories.status = ?",
			product.CategoryName, product.ID, true, true).
		Preload("Variants").Limit(4).Find(&relatedProducts).Error; err != nil {
		pkg.Log.Error("Error fetching related products",
			zap.String("product_id", productID),
			zap.String("category_name", product.CategoryName),
			zap.Error(err))
	}

	pkg.Log.Debug("Fetched related products",
		zap.String("product_id", productID),
		zap.String("category_name", product.CategoryName),
		zap.Int("related_product_count", len(relatedProducts)))

	availableRelatedProducts := []ProductWithOffer{}
	// Pre-fetch all wishlist products for this user to avoid N+1 queries
	wishlistProductIDs := make(map[uint]bool)
	if userID > 0 {
		var wishlistItems []userModels.Wishlist
		if err := database.DB.Where("user_id = ?", userID).
			Select("product_id").
			Find(&wishlistItems).Error; err != nil {
			pkg.Log.Error("Error fetching user wishlist",
				zap.Uint("user_id", userID),
				zap.Error(err))
		} else {
			for _, item := range wishlistItems {
				wishlistProductIDs[item.ProductID] = true
			}
			pkg.Log.Debug("Fetched user wishlist",
				zap.Uint("user_id", userID),
				zap.Int("wishlist_item_count", len(wishlistItems)))
		}
	}

	for _, rp := range relatedProducts {
		bestOfferPrice := float64(999999)
		var selectedOffer ProductWithOffer
		hasValidVariant := false
		for _, v := range rp.Variants {
			if v.Stock > 0 {
				offer := helper.GetBestOfferForProduct(&rp, v.ExtraPrice)
				if offer.IsOfferApplied && offer.DiscountedPrice < bestOfferPrice {
					bestOfferPrice = offer.DiscountedPrice
					selectedOffer = ProductWithOffer{
						Product:            rp,
						OfferPrice:         offer.DiscountedPrice,
						OriginalPrice:      offer.OriginalPrice,
						DiscountPercentage: offer.DiscountPercentage,
						IsOffer:            offer.IsOfferApplied,
						OfferName:          offer.OfferName,
						OfferEndTime:       offer.EndTime,
						IsInWishlist:       wishlistProductIDs[rp.ID],
					}
					hasValidVariant = true
				} else if !offer.IsOfferApplied && (rp.Price+v.ExtraPrice) < bestOfferPrice {
					bestOfferPrice = rp.Price + v.ExtraPrice
					selectedOffer = ProductWithOffer{
						Product:            rp,
						OfferPrice:         rp.Price + v.ExtraPrice,
						OriginalPrice:      rp.Price + v.ExtraPrice,
						DiscountPercentage: 0,
						IsOffer:            false,
						OfferName:          "",
						OfferEndTime:       time.Time{},
						IsInWishlist:       wishlistProductIDs[rp.ID],
					}
					hasValidVariant = true
				}
			}
		}
		// If no valid variant with offer, check if product has stock without offer
		if !hasValidVariant && len(rp.Variants) > 0 {
			for _, v := range rp.Variants {
				if v.Stock > 0 {
					selectedOffer = ProductWithOffer{
						Product:            rp,
						OfferPrice:         rp.Price + v.ExtraPrice,
						OriginalPrice:      rp.Price + v.ExtraPrice,
						DiscountPercentage: 0,
						IsOffer:            false,
						OfferName:          "",
						OfferEndTime:       time.Time{},
						IsInWishlist:       wishlistProductIDs[rp.ID],
					}
					hasValidVariant = true
					break
				}
			}
		}
		if hasValidVariant {
			availableRelatedProducts = append(availableRelatedProducts, selectedOffer)
		}
	}

	pkg.Log.Debug("Processed related products",
		zap.String("product_id", productID),
		zap.Int("available_related_product_count", len(availableRelatedProducts)))

	breadcrumbs := config.GenerateBreadcrumbs(
		config.Breadcrumb{Name: "Shop", URL: "/shop"},
		config.Breadcrumb{Name: product.ProductName, URL: ""},
	)

	var wishlistCount, cartCount int64
	if exists {
		userData := user.(userModels.Users)
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

		pkg.Log.Info("Rendering product details for authenticated user",
			zap.Uint("user_id", userData.ID),
			zap.String("user_name", userNameStr),
			zap.String("product_id", productID),
			zap.String("product_name", product.ProductName),
			zap.String("category_name", product.CategoryName),
			zap.Int("variant_count", len(variantsWithOffer)),
			zap.Int("related_product_count", len(availableRelatedProducts)),
			zap.Bool("is_in_wishlist", isInWishlist),
			zap.Bool("has_stock", hasStock),
			zap.Int64("wishlist_count", wishlistCount),
			zap.Int64("cart_count", cartCount))

		c.HTML(http.StatusOK, "productDetails.html", gin.H{
			"Product":         productWithOffers,
			"RelatedProducts": availableRelatedProducts,
			"Category":        product.CategoryName,
			"HasStock":        hasStock,
			"IsInStock":       hasStock,
			"Breadcrumbs":     breadcrumbs,
			"UserName":        userNameStr,
			"ProfileImage":    userData.ProfileImage,
			"WishlistCount":   wishlistCount,
			"CartCount":       cartCount,
		})
		return
	}

	// Guest user
	pkg.Log.Info("Rendering product details for guest user",
		zap.String("product_id", productID),
		zap.String("product_name", product.ProductName),
		zap.String("category_name", product.CategoryName),
		zap.Int("variant_count", len(variantsWithOffer)),
		zap.Int("related_product_count", len(availableRelatedProducts)),
		zap.Bool("has_stock", hasStock))

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
}
