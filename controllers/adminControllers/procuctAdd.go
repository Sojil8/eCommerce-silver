package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

// ShowAddProductForm renders the add product form with categories
func ShowAddProductForm(c *gin.Context) {
	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.HTML(http.StatusOK, "productAdd.html", gin.H{
		"Product":    nil, // No product data for "Add" mode
		"Categories": categories,
	})
}

// ShowEditProductForm renders the edit product form with categories and product data
func ShowEditProductForm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product adminModels.Product
	if err := database.DB.Preload("Variants").First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.HTML(http.StatusOK, "productAdd.html", gin.H{
		"Product":    &product,
		"Categories": categories,
	})
}

// GetCategoriesAPI provides a JSON endpoint for fetching categories (used by frontend fetchCategories)
func GetCategoriesAPI(c *gin.Context) {
	var categories []adminModels.Category
	if err := database.DB.Where("status = ?", true).Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}
	c.JSON(http.StatusOK, categories)
}

// AddProduct handles adding a new product
func AddProduct(c *gin.Context) {
	middleware.ClearCache()

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	// Validate required fields
	requiredFields := map[string]string{
		"productName":  c.PostForm("productName"),
		"description":  c.PostForm("description"),
		"price":        c.PostForm("price"),
		"categoryName": c.PostForm("categoryName"),
	}
	for field, value := range requiredFields {
		if value == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s is required", field)})
			return
		}
	}

	price, err := strconv.ParseFloat(requiredFields["price"], 64)
	if err != nil || price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format or value"})
		return
	}

	// Validate category
	var category adminModels.Category
	if err := database.DB.Where("category_name = ?", requiredFields["categoryName"]).First(&category).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
		return
	}
	if !category.Status {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Category '%s' is unlisted", requiredFields["categoryName"])})
		return
	}

	// Handle images
	files := form.File["images"]
	if len(files) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
		return
	}
	var imageURLs []string
	for _, file := range files {
		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open image"})
			return
		}
		defer f.Close()
		url, err := helper.ProcessImage(c, f, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process image"})
			return
		}
		imageURLs = append(imageURLs, url)
	}

	// Create product
	product := adminModels.Product{
		ProductName:  requiredFields["productName"],
		Description:  requiredFields["description"],
		Price:        price,
		CategoryName: requiredFields["categoryName"],
		Images:       imageURLs,
		IsListed:     true,
	}

	// Handle variants
	colors := form.Value["color[]"]
	variantPrices := form.Value["variantPrice[]"]
	variantStocks := form.Value["variantStock[]"]
	if len(colors) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one variant is required"})
		return
	}
	if len(colors) != len(variantPrices) || len(colors) != len(variantStocks) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mismatch in variant fields (color, price, stock)"})
		return
	}

	for i, color := range colors {
		extraPrice, err := strconv.ParseFloat(variantPrices[i], 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid extra price for variant %s", color)})
			return
		}
		stock, err := strconv.ParseUint(variantStocks[i], 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid stock for variant %s", color)})
			return
		}
		product.Variants = append(product.Variants, adminModels.Variants{
			Color:      color,
			ExtraPrice: extraPrice,
			Stock:      uint(stock),
		})
	}

	// Save to database
	if err := database.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Product added successfully",
		"product": product,
	})
}

// EditProduct updates an existing product
func EditProduct(c *gin.Context) {
	middleware.ClearCache()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product adminModels.Product
	if err := database.DB.Preload("Variants").First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	// Update fields if provided
	if name := c.PostForm("productName"); name != "" {
		product.ProductName = name
	}
	if desc := c.PostForm("description"); desc != "" {
		product.Description = desc
	}
	if priceStr := c.PostForm("price"); priceStr != "" {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil || price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format or value"})
			return
		}
		product.Price = price
	}
	if categoryName := c.PostForm("categoryName"); categoryName != "" {
		var category adminModels.Category
		if err := database.DB.Where("category_name = ?", categoryName).First(&category).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
			return
		}
		if !category.Status {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Category '%s' is unlisted", categoryName)})
			return
		}
		product.CategoryName = categoryName
	}

	// Handle images
	files := form.File["images"]
	if len(files) > 0 { // New images provided
		if len(files) + len(product.Images) < 3 && len(files) < 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required when updating images"})
			return
		}
		var imagePaths []string
		for _, file := range files {
			openedFile, err := file.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open image file"})
				return
			}
			defer openedFile.Close()
			path, err := helper.ProcessImage(c, openedFile, file)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			imagePaths = append(imagePaths, path)
		}
		// Replace existing images with new ones if provided
		product.Images = imagePaths
	} else {
		// Check if remaining images meet the minimum requirement
		if len(product.Images) < 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
			return
		}
	}

	// Handle color variants update (replace existing variants)
	colors := form.Value["color[]"]
	variantPrices := form.Value["variantPrice[]"]
	variantStocks := form.Value["variantStock[]"]
	if len(colors) > 0 {
		if len(colors) != len(variantPrices) || len(colors) != len(variantStocks) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mismatch in variant fields (color, price, stock)"})
			return
		}
		// Clear existing variants and replace with new ones
		database.DB.Where("product_id = ?", product.ID).Delete(&adminModels.Variants{})
		product.Variants = nil
		for i, color := range colors {
			extraPrice, err := strconv.ParseFloat(variantPrices[i], 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid extra price for variant %s", color)})
				return
			}
			stock, err := strconv.ParseUint(variantStocks[i], 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid stock for variant %s", color)})
				return
			}
			product.Variants = append(product.Variants, adminModels.Variants{
				ProductID:  product.ID,
				Color:      color,
				ExtraPrice: extraPrice,
				Stock:      uint(stock),
			})
		}
	} else if len(product.Variants) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one variant is required"})
		return
	}

	// Save to database
	if err := database.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Product updated successfully",
		"product": product,
	})
}