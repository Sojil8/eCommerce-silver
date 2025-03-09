package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

type product struct {
	Search   string `form:"search"`
	Sort     string `form:"sort"`
	Category string `form:"category"`
}

func GetUserProducts(c *gin.Context) {
	var query product
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	db := database.DB.Model(&adminModels.Product{})
	db = db.Where("is_listed = ?", true)

	if query.Search != "" {
		searchTerm := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(product_name) LIKE ?", searchTerm)
	}

	switch query.Sort {
	case "price_low_to_high":
		db = db.Order("price ASC")
	case "price_high_to_low":
		db = db.Order("price DESC")
	case "a_to_z":
		db = db.Order("product_name ASC")
	case "z_to_a":
		db = db.Order("product_name DESC")
	default:
		db = db.Order("id DESC")
	}

	var products []adminModels.Product
	if err := db.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to Fetch Products",
		})
		return
	}

	// Retrieve username from context
	userName, exists := c.Get("user_name")
	if !exists {
		userName = "Guest"
	}
	fmt.Println("Rendering home.html with UserName:", userName) // Debug log

	c.HTML(http.StatusOK, "home.html", gin.H{
		"status":   "success",
		"Products": products,
		"UserName": userName,
	})
}

// Home page controller
func Home(c *gin.Context) {
	// Featured products query (example)
	var featuredProducts []adminModels.Product
	if err := database.DB.Where("is_listed = ? AND is_featured = ?", true, true).
		Order("id DESC").Limit(8).Find(&featuredProducts).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to fetch featured products",
		})
		return
	}

	c.HTML(http.StatusOK, "home.html", gin.H{
		"Title":            "STORENAME - Quality Products for Everyday Life",
		"FeaturedProducts": featuredProducts,
	})
}
