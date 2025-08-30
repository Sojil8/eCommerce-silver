package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
)

type ShopQuery struct {
	Search   string  `json:"search"`
	Sort     string  `json:"sort"`
	Category string  `json:"category"`
	PriceMin float64 `json:"price_min"`
	PriceMax float64 `json:"price_max"`
}

type ShopProduct struct {
	adminModels.Product
	IsOffer            bool    `json:"is_offer"`
	OfferPrice         float64 `json:"offer_price"`
	OriginalPrice      float64 `json:"original_price"`
	DiscountPercentage float64 `json:"discount_percentage"`
	OfferName          string  `json:"offer_name"`
}

func GetUserShop(c *gin.Context) {
	var query ShopQuery
	if c.Request.Method == "POST" {
		if err := c.ShouldBindJSON(&query); err != nil {
			query.Search = c.Query("search")
			query.Sort = c.Query("sort")
			query.Category = c.Query("category")
			if min := c.Query("price_min"); min != "" {
				json.Unmarshal([]byte(min), &query.PriceMin)
			}
			if max := c.Query("price_max"); max != "" {
				json.Unmarshal([]byte(max), &query.PriceMax)
			}
		}
	} else {
		query.Search = c.Query("search")
		query.Sort = c.Query("sort")
		query.Category = c.Query("category")
		if min := c.Query("price_min"); min != "" {
			json.Unmarshal([]byte(min), &query.PriceMin)
		}
		if max := c.Query("price_max"); max != "" {
			json.Unmarshal([]byte(max), &query.PriceMax)
		}
	}

	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed=? AND categories.status = ?", true, true).
		Preload("Variants")

	if query.Search != "" {
		searchTerm := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(products.product_name) LIKE ? OR LOWER(products.description) LIKE ?", searchTerm, searchTerm)
	}

	if query.Category != "" {
		db = db.Where("products.category_name = ?", query.Category)
	}

	if query.PriceMin > 0 {
		db = db.Where("products.price >= ?", query.PriceMin)
	}

	if query.PriceMax > 0 {
		db = db.Where("products.price <= ?", query.PriceMax)
	}

	switch query.Sort {
	case "price_low_to_high":
		db = db.Order("products.price ASC")
	case "price_high_to_low":
		db = db.Order("products.price DESC")
	case "a_to_z":
		db = db.Order("products.product_name ASC")
	case "z_to_a":
		db = db.Order("products.product_name DESC")
	default:
		db = db.Order("products.id DESC")
	}

	var products []adminModels.Product
	if err := db.Find(&products).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:failed to fetch products", "error:failed to fetch products", "")
		return
	}

	var availableProducts []adminModels.Product
	for _, p := range products {
		if p.IsListed {
			availableProducts = append(availableProducts, p)
		}

	}

	var shopProducts []ShopProduct
	for _, product := range availableProducts {
		var variants adminModels.Variants
		if err := database.DB.Find(&variants, product.Variants).Error; err != nil {
			helper.ResponseWithErr(c, http.StatusNotFound, "Product varinats not found", "", "")
			return
		}

		offerDetails := helper.GetBestOfferForProduct(&product, variants.ExtraPrice)

		shopProduct := ShopProduct{
			Product:            product,
			IsOffer:            offerDetails.IsOfferApplied,
			OfferPrice:         offerDetails.DiscountedPrice,
			OriginalPrice:      offerDetails.OriginalPrice,
			DiscountPercentage: offerDetails.DiscountPercentage,
			OfferName:          offerDetails.OfferName,
		}
		shopProducts = append(shopProducts, shopProduct)
	}

	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:Failed to fetch categories", "error:Failed to fetch categories", "")
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "shop.html", gin.H{
			"Products":      shopProducts,
			"Categories":    categories,
			"Query":         query,
			"status":        "success",
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

	c.HTML(http.StatusOK, "shop.html", gin.H{
		"Products":      shopProducts,
		"Categories":    categories,
		"Query":         query,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}
