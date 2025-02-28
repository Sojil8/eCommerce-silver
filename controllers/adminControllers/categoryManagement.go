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

	var categorys []adminModels.Category
	var total int64

	database.DB.Unscoped().Model(&adminModels.Category{}).Count(&total)
	if err := database.DB.Unscoped().Order("category_name").Offset(offset).Limit(itemsPerPage).Find(&categorys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch categories",
		})
		return
	}

	totalPages := (int(total) + itemsPerPage - 1) / itemsPerPage

	categoryData := make([]map[string]interface{}, len(categorys))
	for i, cat := range categorys {
		categoryData[i] = map[string]interface{}{
			"_id":           cat.ID,
			"name":          cat.Category_name,
			"description":   cat.Description,
			"categoryOffer": 0,
			"isListed":      cat.DeletedAt.Valid == false,
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

	if !category.Status {
		category.Status = true
	}

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

	var intput struct {
		CategoryName string `json:"category_name"`
		Description  string `json:"description"`
	}
	if err := c.ShouldBindJSON(&intput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid input: " + err.Error(),
		})
		return
	}

	category.Category_name = intput.CategoryName
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
