package controllers

import (
	"log"
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
	if exists {
		userData := user.(userModels.Users)
		var count int64
		if err := database.DB.Model(&userModels.Wishlist{}).
			Where("user_id = ? AND product_id = ?", userData.ID, product.ID).
			Count(&count).Error; err != nil {
			pkg.Log.Error("Error checking wishlist", zap.Error(err))
		} else {
			isInWishlist = count > 0
		}
		productWithOffers.IsInWishlist = isInWishlist
	}

	pkg.Log.Info("Product with offer",
		zap.Any("Product", productWithOffers.Product),
		zap.Float64("OfferPrice", productWithOffers.OfferPrice),
		zap.Float64("OriginalPrice", productWithOffers.OriginalPrice),
		zap.Float64("DiscountPercentage", productWithOffers.DiscountPercentage),
		zap.Bool("IsOffer", productWithOffers.IsOffer),
		zap.String("OfferName", productWithOffers.OfferName),
		zap.Time("OfferEndTime", productWithOffers.OfferEndTime),
		zap.Bool("IsInWishlist", isInWishlist),
	)

	var relatedProducts []adminModels.Product
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.category_name = ? AND products.id != ? AND products.is_listed = ? AND categories.status = ?",
			product.CategoryName, product.ID, true, true).
		Preload("Variants").Limit(4).Find(&relatedProducts).Error; err != nil {
		log.Println("Error fetching related products:", err)
	}

	availableRelatedProducts := []ProductWithOffer{}
	userID := uint(0)
	if exists {
		userData := user.(userModels.Users)
		userID = userData.ID
	}

	// Pre-fetch all wishlist products for this user to avoid N+1 queries
	wishlistProductIDs := make(map[uint]bool)
	if userID > 0 {
		var wishlistItems []userModels.Wishlist
		if err := database.DB.Where("user_id = ?", userID).
			Select("product_id").
			Find(&wishlistItems).Error; err != nil {
			pkg.Log.Error("Error fetching user wishlist", zap.Error(err))
		} else {
			for _, item := range wishlistItems {
				wishlistProductIDs[item.ProductID] = true
			}
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
						IsInWishlist:       wishlistProductIDs[rp.ID], // Check wishlist status
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
						IsInWishlist:       wishlistProductIDs[rp.ID], // Check wishlist status
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

	breadcrumbs := config.GenerateBreadcrumbs(
		config.Breadcrumb{Name: "Shop", URL: "/shop"},
		config.Breadcrumb{Name: product.ProductName, URL: ""},
	)

	userName, _ := c.Get("user_name")
	var wishlistCount, cartCount int64
	if exists {
		userData := user.(userModels.Users)
		if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
			wishlistCount = 0
		}
		if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
			cartCount = 0
		}
		userNameStr := userName.(string)
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