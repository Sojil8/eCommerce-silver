package controllers

import (
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GetCategories(c *gin.Context) {
	pkg.Log.Info("Handling request to get categories")

	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		pkg.Log.Warn("Invalid or missing page parameter, defaulting to 1", zap.String("page", pageStr), zap.Error(err))
		page = 1
	}
	pkg.Log.Debug("Parsed page parameter", zap.Int("page", page))

	const itemsPerPage = 10
	offset := (page - 1) * itemsPerPage

	var categories []adminModels.Category
	var total int64

	if err := database.DB.Model(&adminModels.Category{}).Count(&total).Error; err != nil {
		pkg.Log.Error("Failed to count categories", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch categories",
		})
		return
	}
	pkg.Log.Debug("Total categories counted", zap.Int64("total", total))

	if err := database.DB.Order("category_name").Offset(offset).Limit(itemsPerPage).Find(&categories).Error; err != nil {
		pkg.Log.Error("Failed to fetch categories", zap.Int("offset", offset), zap.Int("limit", itemsPerPage), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch categories",
		})
		return
	}
	pkg.Log.Debug("Categories retrieved", zap.Int("count", len(categories)))

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
		pkg.Log.Debug("Category processed",
			zap.Uint("id", cat.ID),
			zap.String("name", cat.CategoryName),
			zap.Bool("status", cat.Status))
	}

	pkg.Log.Info("Rendering categoryManagement.html", zap.Int("total_pages", totalPages), zap.Int("category_count", len(categoryData)))
	c.HTML(http.StatusOK, "categoryManagement.html", gin.H{
		"cat":        categoryData,
		"totalPages": totalPages,
	})
}

func AddCategory(c *gin.Context) {
	pkg.Log.Info("Handling request to add category")

	var category adminModels.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	pkg.Log.Debug("Received category data", zap.String("name", category.CategoryName), zap.String("description", category.Description))

	category.Status = true
	if err := database.DB.Create(&category).Error; err != nil {
		pkg.Log.Error("Failed to create category", zap.String("name", category.CategoryName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	pkg.Log.Info("Category created successfully", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))
	c.JSON(http.StatusOK, gin.H{
		"message": "Category Created",
	})
}

var input struct {
	CategoryName string `json:"category_name"`
	Description  string `json:"description"`
}

func EditCategory(c *gin.Context) {
	pkg.Log.Info("Handling request to edit category")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.Log.Warn("Invalid category ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid category ID",
		})
		return
	}
	pkg.Log.Debug("Parsed category ID", zap.Int("id", id))

	var category adminModels.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		pkg.Log.Error("Category not found", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Category not found",
		})
		return
	}
	pkg.Log.Debug("Category retrieved", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))

	if err := c.ShouldBindJSON(&input); err != nil {
		pkg.Log.Error("Failed to bind JSON input", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid input: " + err.Error(),
		})
		return
	}
	pkg.Log.Debug("Received update data", zap.String("name", input.CategoryName), zap.String("description", input.Description))

	category.CategoryName = input.CategoryName
	category.Description = input.Description

	if err := database.DB.Save(&category).Error; err != nil {
		pkg.Log.Error("Failed to update category", zap.Uint("id", category.ID), zap.String("name", category.CategoryName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to update category: " + err.Error(),
		})
		return
	}

	pkg.Log.Info("Category updated successfully", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))
	c.JSON(http.StatusOK, gin.H{
		"message": "Category updated successfully",
	})
}

func ListCategory(c *gin.Context) {
	pkg.Log.Info("Handling request to list category")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.Log.Warn("Invalid category ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}
	pkg.Log.Debug("Parsed category ID", zap.Int("id", id))

	var category adminModels.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		pkg.Log.Error("Category not found", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}
	pkg.Log.Debug("Category retrieved", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))

	category.Status = true
	if err := database.DB.Save(&category).Error; err != nil {
		pkg.Log.Error("Failed to list category", zap.Uint("id", category.ID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list category"})
		return
	}

	pkg.Log.Info("Category listed successfully", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))
	c.JSON(http.StatusOK, gin.H{
		"message":  "Category listed successfully",
		"category": category,
	})
}

func UnlistCategory(c *gin.Context) {
	pkg.Log.Info("Handling request to unlist category")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.Log.Warn("Invalid category ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}
	pkg.Log.Debug("Parsed category ID", zap.Int("id", id))

	var category adminModels.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		pkg.Log.Error("Category not found", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Category not found",
		})
		return
	}
	pkg.Log.Debug("Category retrieved", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))

	category.Status = false
	if err := database.DB.Save(&category).Error; err != nil {
		pkg.Log.Error("Failed to unlist category", zap.Uint("id", category.ID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlist category"})
		return
	}

	pkg.Log.Info("Category unlisted successfully", zap.Uint("id", category.ID), zap.String("name", category.CategoryName))
	c.JSON(http.StatusOK, gin.H{"message": "Category unlisted successfully"})
}
