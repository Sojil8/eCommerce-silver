package controllers

import (
	"net/http"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

type productQuery struct {
	Search   string `form:"search"`
	Sort     string `form:"sort"`
	Category string `form:"category"`
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

	userName, exists := c.Get("user_name")
	if !exists {
		userName = "Guest"
	}

	c.HTML(http.StatusOK, "home.html", gin.H{
		"status":   "success",
		"Products": products,
		"UserName": userName,
	})
}

func Home(c *gin.Context) {
	var featuredProducts []adminModels.Product
	if err := database.DB.Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed = ? AND products.is_featured = ? AND categories.status = ?", true, true, true).
		Order("products.id DESC").Limit(8).Preload("Variants").Find(&featuredProducts).Error; err != nil {
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
