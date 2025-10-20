package controllers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
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
	Brand    string  `json:"brand"` // Added Brand field
	PriceMin float64 `json:"price_min"`
	PriceMax float64 `json:"price_max"`
	Page     int     `json:"page"`
	Limit    int     `json:"limit"`
}

type ShopProduct struct {
	adminModels.Product
	IsOffer            bool    `json:"is_offer"`
	OfferPrice         float64 `json:"offer_price"`
	OriginalPrice      float64 `json:"original_price"`
	DiscountPercentage float64 `json:"discount_percentage"`
	OfferName          string  `json:"offer_name"`
}

type PaginationData struct {
	CurrentPage  int
	TotalPages   int
	TotalItems   int64
	ItemsPerPage int
	HasPrevious  bool
	HasNext      bool
	StartItem    int
	EndItem      int
}

func GetUserShop(c *gin.Context) {
	var query ShopQuery
	if c.Request.Method == "POST" {
		if err := c.ShouldBindJSON(&query); err != nil {
			query.Search = c.Query("search")
			query.Sort = c.Query("sort")
			query.Category = c.Query("category")
			query.Brand = c.Query("brand") // Added Brand query parameter
			if min := c.Query("price_min"); min != "" {
				json.Unmarshal([]byte(min), &query.PriceMin)
			}
			if max := c.Query("price_max"); max != "" {
				json.Unmarshal([]byte(max), &query.PriceMax)
			}
			if page := c.Query("page"); page != "" {
				query.Page, _ = strconv.Atoi(page)
			}
			if limit := c.Query("limit"); limit != "" {
				query.Limit, _ = strconv.Atoi(limit)
			}
		}
	} else {
		query.Search = c.Query("search")
		query.Sort = c.Query("sort")
		query.Category = c.Query("category")
		query.Brand = c.Query("brand") // Added Brand query parameter
		if min := c.Query("price_min"); min != "" {
			json.Unmarshal([]byte(min), &query.PriceMin)
		}
		if max := c.Query("price_max"); max != "" {
			json.Unmarshal([]byte(max), &query.PriceMax)
		}
		if page := c.Query("page"); page != "" {
			query.Page, _ = strconv.Atoi(page)
		}
		if limit := c.Query("limit"); limit != "" {
			query.Limit, _ = strconv.Atoi(limit)
		}
	}

	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 6
	}

	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed=? AND categories.status = ?", true, true).
		Preload("Variants")

	if query.Search != "" {
		searchTerm := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(products.product_name) LIKE ? OR LOWER(products.description) LIKE ? OR LOWER(products.brand) LIKE ?", searchTerm, searchTerm, searchTerm) // Added brand to search
	}

	if query.Category != "" {
		db = db.Where("products.category_name = ?", query.Category)
	}

	if query.Brand != "" {
		db = db.Where("products.brand = ?", query.Brand) // Added brand filtering
	}

	// Adjust price filtering to account for variant extra price
	if query.PriceMin > 0 {
		db = db.Where("(products.price + COALESCE((SELECT variants.extra_price FROM variants WHERE variants.product_id = products.id LIMIT 1), 0)) >= ?", query.PriceMin)
	}

	if query.PriceMax > 0 {
		db = db.Where("(products.price + COALESCE((SELECT variants.extra_price FROM variants WHERE variants.product_id = products.id LIMIT 1), 0)) <= ?", query.PriceMax)
	}

	// Get total count before pagination
	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:failed to count products", "error:failed to count products", "")
		return
	}

	// Fetch all filtered products (no limit/offset/order here, we'll handle sorting and pagination in memory)
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
		variantExtraPrice := 0.0
		if len(product.Variants) > 0 {
			variantExtraPrice = product.Variants[0].ExtraPrice
		}

		offerDetails := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		// Calculate original price including variant extra price
		originalPrice := product.Price
		if len(product.Variants) > 0 {
			originalPrice += product.Variants[0].ExtraPrice
		}

		shopProduct := ShopProduct{
			Product:            product,
			IsOffer:            offerDetails.IsOfferApplied,
			OfferPrice:         offerDetails.DiscountedPrice,
			OriginalPrice:      originalPrice,
			DiscountPercentage: offerDetails.DiscountPercentage,
			OfferName:          offerDetails.OfferName,
		}
		shopProducts = append(shopProducts, shopProduct)
	}

	// Now sort the shopProducts in memory based on the query.Sort, using the final price (offer if applicable)
	switch query.Sort {
	case "price_low_to_high":
		sort.Slice(shopProducts, func(i, j int) bool {
			priceI := shopProducts[i].OfferPrice
			if !shopProducts[i].IsOffer {
				priceI = shopProducts[i].OriginalPrice
			}
			priceJ := shopProducts[j].OfferPrice
			if !shopProducts[j].IsOffer {
				priceJ = shopProducts[j].OriginalPrice
			}
			return priceI < priceJ
		})
	case "price_high_to_low":
		sort.Slice(shopProducts, func(i, j int) bool {
			priceI := shopProducts[i].OfferPrice
			if !shopProducts[i].IsOffer {
				priceI = shopProducts[i].OriginalPrice
			}
			priceJ := shopProducts[j].OfferPrice
			if !shopProducts[j].IsOffer {
				priceJ = shopProducts[j].OriginalPrice
			}
			return priceI > priceJ
		})
	case "a_to_z":
		sort.Slice(shopProducts, func(i, j int) bool {
			return shopProducts[i].ProductName < shopProducts[j].ProductName
		})
	case "z_to_a":
		sort.Slice(shopProducts, func(i, j int) bool {
			return shopProducts[i].ProductName > shopProducts[j].ProductName
		})
	default:
		sort.Slice(shopProducts, func(i, j int) bool {
			return shopProducts[i].ID > shopProducts[j].ID // Default: newest first (ID DESC)
		})
	}

	// Update totalCount and pagination based on the sorted shopProducts
	totalCount = int64(len(shopProducts))
	totalPages := int((totalCount + int64(query.Limit) - 1) / int64(query.Limit))

	// Apply in-memory pagination
	offset := (query.Page - 1) * query.Limit
	if offset > len(shopProducts) {
		offset = len(shopProducts)
	}
	end := offset + query.Limit
	if end > len(shopProducts) {
		end = len(shopProducts)
	}
	paginatedShopProducts := shopProducts[offset:end]

	// Calculate pagination data
	startItem := offset + 1
	endItem := offset + len(paginatedShopProducts)
	if totalCount == 0 {
		startItem = 0
	}

	pagination := PaginationData{
		CurrentPage:  query.Page,
		TotalPages:   totalPages,
		TotalItems:   totalCount,
		ItemsPerPage: query.Limit,
		HasPrevious:  query.Page > 1,
		HasNext:      query.Page < totalPages,
		StartItem:    startItem,
		EndItem:      endItem,
	}

	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:Failed to fetch categories", "error:Failed to fetch categories", "")
		return
	}

	// Fetch distinct brands
	var brands []string
	if err := database.DB.Model(&adminModels.Product{}).Where("is_listed = ?", true).Distinct("brand").Pluck("brand", &brands).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:Failed to fetch brands", "error:Failed to fetch brands", "")
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "shop.html", gin.H{
			"Products":      paginatedShopProducts,
			"Categories":    categories,
			"Brands":        brands, // Added Brands to template data
			"Query":         query,
			"Pagination":    pagination,
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
		"Products":      paginatedShopProducts,
		"Categories":    categories,
		"Brands":        brands, // Added Brands to template data
		"Query":         query,
		"Pagination":    pagination,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}