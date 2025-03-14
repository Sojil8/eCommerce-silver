package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

type ShopQuery struct {
	Search   string  `json:"search"`
	Sort     string  `json:"sort"`
	Category string  `json:"category"`
	PriceMin float64 `json:"price_min"`
	PriceMax float64 `json:"price_max"`
}

func GetUserShop(c *gin.Context) {
	var query ShopQuery
	if c.Request.Method == "POST" {
		if err := c.ShouldBindJSON(&query); err != nil {
			// If JSON binding fails, try to get from query parameters
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
		// For GET requests, get from query parameters
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
		// Fixed: Use <= for maximum price
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
	// Fixed: Use the prepared query with all filters
	if err := db.Find(&products).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:failed to fetch products", "error:failed to fetch products", "")
		return
	}

	var availableProducts []adminModels.Product
	for _, p := range products {
		for _, v := range p.Variants {
			if v.Stock > 0 {
				availableProducts = append(availableProducts, p)
				break
			}
		}
	}

	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:Failed to fetch categories", "error:Failed to fetch categories", "")
		return
	}

	userName, exits := c.Get("user_name")
	if !exits {
		userName = "Guest"
	}

	c.HTML(http.StatusOK, "shop.html", gin.H{
		"Products":   availableProducts,
		"Categories": categories,
		"Query":      query,
		"UserName":   userName,
	})
}
