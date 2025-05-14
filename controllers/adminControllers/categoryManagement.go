package controllers

import (
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

func GetCategories(c *gin.Context) {
	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	const itemsPerPage = 10
	offset := (page - 1) * itemsPerPage

	var categories []adminModels.Category
	var total int64

	database.DB.Model(&adminModels.Category{}).Count(&total)
	if err := database.DB.Order("category_name").Offset(offset).Limit(itemsPerPage).Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch categories",
		})
		return
	}

	totalPages := (int(total) + itemsPerPage - 1) / itemsPerPage
	pageRange := make([]int, totalPages)
	for i := 0; i < totalPages; i++ {
		pageRange[i] = i + 1
	}

	categoryData := make([]map[string]interface{}, len(categories))
	for i, cat := range categories {
		categoryData[i] = map[string]interface{}{
			"_id":           cat.ID,
			"name":          cat.CategoryName,
			"description":   cat.Description,
			"categoryOffer": 0,
			"isListed":      cat.Status,
		}
	}

	c.HTML(http.StatusOK, "categoryManagement.html", gin.H{
		"cat":        categoryData,
		"totalPages": totalPages,
	})
}

func AddCategory(c *gin.Context) {
	var category adminModels.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	category.Status = true
	if err := database.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Category Created",
	})
}

var intput struct {
	CategoryName string `json:"category_name"`
	Description  string `json:"description"`
}

func EditCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid category ID",
		})
		return
	}

	var category adminModels.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Category not found",
		})
		return
	}

	if err := c.ShouldBindJSON(&intput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid input: " + err.Error(),
		})
		return
	}

	category.CategoryName = intput.CategoryName
	category.Description = intput.Description

	if err := database.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to update category: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Category updated successfully",
	})
}

func ListCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	var category adminModels.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	category.Status = true
	if err := database.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Category listed successfully",
		"category": category,
	})
}

func UnlistCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	var category adminModels.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Category not found",
		})
		return
	}

	category.Status = false
	if err := database.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Category listed successfully"})
}
