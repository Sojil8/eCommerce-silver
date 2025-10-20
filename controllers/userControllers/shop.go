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
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ShopQuery struct {
	Search   string  `json:"search"`
	Sort     string  `json:"sort"`
	Category string  `json:"category"`
	Brand    string  `json:"brand"`
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
	pkg.Log.Info("Starting shop page retrieval")

	var query ShopQuery
	if c.Request.Method == "POST" {
		if err := c.ShouldBindJSON(&query); err != nil {
			pkg.Log.Warn("Failed to bind JSON query, falling back to query parameters",
				zap.Error(err))
			query.Search = c.Query("search")
			query.Sort = c.Query("sort")
			query.Category = c.Query("category")
			query.Brand = c.Query("brand")
			if min := c.Query("price_min"); min != "" {
				if err := json.Unmarshal([]byte(min), &query.PriceMin); err != nil {
					pkg.Log.Warn("Invalid price_min parameter",
						zap.String("price_min", min),
						zap.Error(err))
				}
			}
			if max := c.Query("price_max"); max != "" {
				if err := json.Unmarshal([]byte(max), &query.PriceMax); err != nil {
					pkg.Log.Warn("Invalid price_max parameter",
						zap.String("price_max", max),
						zap.Error(err))
				}
			}
			if page := c.Query("page"); page != "" {
				if pageNum, err := strconv.Atoi(page); err == nil {
					query.Page = pageNum
				} else {
					pkg.Log.Warn("Invalid page parameter",
						zap.String("page", page),
						zap.Error(err))
				}
			}
			if limit := c.Query("limit"); limit != "" {
				if limitNum, err := strconv.Atoi(limit); err == nil {
					query.Limit = limitNum
				} else {
					pkg.Log.Warn("Invalid limit parameter",
						zap.String("limit", limit),
						zap.Error(err))
				}
			}
		}
	} else {
		query.Search = c.Query("search")
		query.Sort = c.Query("sort")
		query.Category = c.Query("category")
		query.Brand = c.Query("brand")
		if min := c.Query("price_min"); min != "" {
			if err := json.Unmarshal([]byte(min), &query.PriceMin); err != nil {
				pkg.Log.Warn("Invalid price_min parameter",
					zap.String("price_min", min),
					zap.Error(err))
			}
		}
		if max := c.Query("price_max"); max != "" {
			if err := json.Unmarshal([]byte(max), &query.PriceMax); err != nil {
				pkg.Log.Warn("Invalid price_max parameter",
					zap.String("price_max", max),
					zap.Error(err))
			}
		}
		if page := c.Query("page"); page != "" {
			if pageNum, err := strconv.Atoi(page); err == nil {
				query.Page = pageNum
			} else {
				pkg.Log.Warn("Invalid page parameter",
					zap.String("page", page),
					zap.Error(err))
			}
		}
		if limit := c.Query("limit"); limit != "" {
			if limitNum, err := strconv.Atoi(limit); err == nil {
				query.Limit = limitNum
			} else {
				pkg.Log.Warn("Invalid limit parameter",
					zap.String("limit", limit),
					zap.Error(err))
			}
		}
	}

	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 6
	}

	pkg.Log.Debug("Processed shop query",
		zap.String("search_term", query.Search),
		zap.String("sort", query.Sort),
		zap.String("category", query.Category),
		zap.String("brand", query.Brand),
		zap.Float64("price_min", query.PriceMin),
		zap.Float64("price_max", query.PriceMax),
		zap.Int("page", query.Page),
		zap.Int("limit", query.Limit))

	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed=? AND categories.status = ?", true, true).
		Preload("Variants")

	if query.Search != "" {
		searchTerm := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(products.product_name) LIKE ? OR LOWER(products.description) LIKE ? OR LOWER(products.brand) LIKE ?", searchTerm, searchTerm, searchTerm)
	}

	if query.Category != "" {
		db = db.Where("products.category_name = ?", query.Category)
	}

	if query.Brand != "" {
		db = db.Where("products.brand = ?", query.Brand)
	}

	if query.PriceMin > 0 {
		db = db.Where("(products.price + COALESCE((SELECT variants.extra_price FROM variants WHERE variants.product_id = products.id LIMIT 1), 0)) >= ?", query.PriceMin)
	}

	if query.PriceMax > 0 {
		db = db.Where("(products.price + COALESCE((SELECT variants.extra_price FROM variants WHERE variants.product_id = products.id LIMIT 1), 0)) <= ?", query.PriceMax)
	}

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		pkg.Log.Error("Failed to count products",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:failed to count products", "error:failed to count products", "")
		return
	}

	pkg.Log.Debug("Counted total products",
		zap.Int64("total_count", totalCount))

	var products []adminModels.Product
	if err := db.Find(&products).Error; err != nil {
		pkg.Log.Error("Failed to fetch products",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:failed to fetch products", "error:failed to fetch products", "")
		return
	}

	var availableProducts []adminModels.Product
	for _, p := range products {
		if p.IsListed {
			availableProducts = append(availableProducts, p)
		}
	}

	pkg.Log.Debug("Filtered available products",
		zap.Int("available_product_count", len(availableProducts)))

	var shopProducts []ShopProduct
	for _, product := range availableProducts {
		variantExtraPrice := 0.0
		if len(product.Variants) > 0 {
			variantExtraPrice = product.Variants[0].ExtraPrice
		}

		offerDetails := helper.GetBestOfferForProduct(&product, variantExtraPrice)

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

	pkg.Log.Debug("Processed shop products with offers",
		zap.Int("shop_product_count", len(shopProducts)))

	switch query.Sort {
	case "price_low_to_high":
		pkg.Log.Debug("Sorting products by price low to high")
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
		pkg.Log.Debug("Sorting products by price high to low")
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
		pkg.Log.Debug("Sorting products A to Z")
		sort.Slice(shopProducts, func(i, j int) bool {
			return shopProducts[i].ProductName < shopProducts[j].ProductName
		})
	case "z_to_a":
		pkg.Log.Debug("Sorting products Z to A")
		sort.Slice(shopProducts, func(i, j int) bool {
			return shopProducts[i].ProductName > shopProducts[j].ProductName
		})
	default:
		pkg.Log.Debug("Sorting products by default (newest first)")
		sort.Slice(shopProducts, func(i, j int) bool {
			return shopProducts[i].ID > shopProducts[j].ID
		})
	}

	totalCount = int64(len(shopProducts))
	totalPages := int((totalCount + int64(query.Limit) - 1) / int64(query.Limit))

	offset := (query.Page - 1) * query.Limit
	if offset > len(shopProducts) {
		offset = len(shopProducts)
	}
	end := offset + query.Limit
	if end > len(shopProducts) {
		end = len(shopProducts)
	}
	paginatedShopProducts := shopProducts[offset:end]

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

	pkg.Log.Debug("Processed pagination",
		zap.Int("current_page", pagination.CurrentPage),
		zap.Int("total_pages", pagination.TotalPages),
		zap.Int64("total_items", pagination.TotalItems),
		zap.Int("items_per_page", pagination.ItemsPerPage),
		zap.Int("start_item", pagination.StartItem),
		zap.Int("end_item", pagination.EndItem))

	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		pkg.Log.Error("Failed to fetch categories",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:Failed to fetch categories", "error:Failed to fetch categories", "")
		return
	}

	pkg.Log.Debug("Fetched categories",
		zap.Int("category_count", len(categories)))

	var brands []string
	if err := database.DB.Model(&adminModels.Product{}).Where("is_listed = ?", true).Distinct("brand").Pluck("brand", &brands).Error; err != nil {
		pkg.Log.Error("Failed to fetch brands",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:Failed to fetch brands", "error:Failed to fetch brands", "")
		return
	}

	pkg.Log.Debug("Fetched brands",
		zap.Int("brand_count", len(brands)))

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Info("Rendering shop page for guest user",
			zap.Int("product_count", len(paginatedShopProducts)),
			zap.Int("category_count", len(categories)),
			zap.Int("brand_count", len(brands)),
			zap.Int("total_pages", totalPages))

		c.HTML(http.StatusOK, "shop.html", gin.H{
			"Products":      paginatedShopProducts,
			"Categories":    categories,
			"Brands":        brands,
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
	userID := userData.ID

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch wishlist count",
			zap.Uint("user_id", userID),
			zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count",
			zap.Uint("user_id", userID),
			zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Info("Rendering shop page for authenticated user",
		zap.Uint("user_id", userID),
		zap.String("user_name", userNameStr),
		zap.Int("product_count", len(paginatedShopProducts)),
		zap.Int("category_count", len(categories)),
		zap.Int("brand_count", len(brands)),
		zap.Int("total_pages", totalPages),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount))

	c.HTML(http.StatusOK, "shop.html", gin.H{
		"Products":      paginatedShopProducts,
		"Categories":    categories,
		"Brands":        brands,
		"Query":         query,
		"Pagination":    pagination,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}
