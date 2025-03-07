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

func GetProducts(c *gin.Context) {
	middleware.ClearCache()
	pageStr := c.Query("page")
	searchQuery := c.Query("search")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	const itemsPerPage = 10
	offset := (page - 1) * itemsPerPage

	var products []adminModels.Product
	var total int64

	dbQuery := database.DB.Model(&adminModels.Product{})

	// Apply search filter if query exists
	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		dbQuery = dbQuery.Where("product_name ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	// Count total items (filtered or not)
	if err := dbQuery.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
		return
	}

	// Fetch paginated results
	if err := dbQuery.Order("product_name").Offset(offset).Limit(itemsPerPage).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	totalPages := (int(total) + itemsPerPage - 1) / itemsPerPage

	c.HTML(http.StatusOK, "product.html", gin.H{
		"Products":    products,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"SearchQuery": searchQuery, // Pass search query to maintain state
	})
}

func AddProduct(c *gin.Context) {
	middleware.ClearCache()
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Failed to parse form data",
			"error":   err.Error(),
		})
		return
	}

	name := form.Value["productName"][0]
	description := form.Value["description"][0]
	priceStr := form.Value["price"][0]
	categoryIdStr := form.Value["category_id"][0]

	if name == "" || description == "" || priceStr == "" || categoryIdStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "All fields are required",
		})
		return
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid price format",
		})
		return
	}

	categoryID, err := strconv.Atoi(categoryIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid category ID",
		})
		return
	}

	files := form.File["images"]
	if len(files) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "At least 3 images are required",
		})
		return
	}

	var imageURLs []string
	for _, file := range files {
		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to open image",
				"error":   err.Error(),
			})
			return
		}
		defer f.Close()

		// Pass the Gin context (c) to ProcessImage
		url, err := helper.ProcessImage(c, f, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to process image",
				"error":   err.Error(),
			})
			return
		}
		imageURLs = append(imageURLs, url)
	}

	product := adminModels.Product{
		ProductName: name,
		Description: description,
		Price:       price,
		Category_id: uint(categoryID),
		Images:      imageURLs,
	}

	if err := database.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create product",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Product added successfully",
		"product": product,
	})
}

func EditProduct(c *gin.Context) {
	middleware.ClearCache()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product adminModels.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}
	if name := c.PostForm("productName"); name != "" {
		product.ProductName = name
	}
	if desc := c.PostForm("description"); desc != "" {
		product.Description = desc
	}
	if priceStr := c.PostForm("price"); priceStr != "" {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format"})
			return
		}
		product.Price = price
	}
	if catIDStr := c.PostForm("category_id"); catIDStr != "" {
		catID, err := strconv.Atoi(catIDStr)
		if err != nil || catID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
			return
		}
		product.Category_id = uint(catID)
	}

	if files := form.File["images"]; len(files) > 0 {
		if len(files) < 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required for update"})
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

			// Pass the Gin context (c) to ProcessImage
			path, err := helper.ProcessImage(c, openedFile, file)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			imagePaths = append(imagePaths, path)
		}
		product.Images = imagePaths
	}
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

func ToggleProductStatus(c *gin.Context) {
	middleware.ClearCache()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product adminModels.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	product.IsListed = !product.IsListed
	if err := database.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle product status"})
		return
	}

	status := "listed"
	if !product.IsListed {
		status = "unlisted"
	}
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Product %s successfully", status),
	})
}
