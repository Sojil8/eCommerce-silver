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
	fmt.Println("GetProducts handler called")
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

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		// Updated to include CategoryName in search if desired
		dbQuery = dbQuery.Where("product_name ILIKE ? OR description ILIKE ? OR category_name ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
		return
	}

	// Preload Category if you have a relationship, otherwise just fetch products
	if err := dbQuery.Order("product_name").Offset(offset).Limit(itemsPerPage).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	totalPages := (int(total) + itemsPerPage - 1) / itemsPerPage

	c.HTML(http.StatusOK, "product.html", gin.H{
		"Products":    products,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"SearchQuery": searchQuery,
	})
}
func AddProduct(c *gin.Context) {
    middleware.ClearCache()
    form, err := c.MultipartForm()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
        return
    }

    // Add quantity field
    quantityValues, qtyOk := form.Value["quantity"]
    nameValues, nameOk := form.Value["productName"]
    descriptionValues, descOk := form.Value["description"]
    priceValues, priceOk := form.Value["price"]
    categoryValues, catOk := form.Value["categoryName"]

    if !nameOk || len(nameValues) == 0 || !descOk || len(descriptionValues) == 0 ||
       !priceOk || len(priceValues) == 0 || !catOk || len(categoryValues) == 0 ||
       !qtyOk || len(quantityValues) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
        return
    }

    name := nameValues[0]
    description := descriptionValues[0]
    priceStr := priceValues[0]
    categoryName := categoryValues[0]
    quantityStr := quantityValues[0]

    price, err := strconv.ParseFloat(priceStr, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format"})
        return
    }

    quantity, err := strconv.ParseUint(quantityStr, 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid quantity format"})
        return
    }

    // Category validation remains the same
    var category adminModels.Category
    if err := database.DB.Where("category_name = ?", categoryName).First(&category).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
        return
    }
    if !category.Status {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": fmt.Sprintf("Category '%s' is unlisted and cannot be used", categoryName),
        })
        return
    }

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

    product := adminModels.Product{
        ProductName:  name,
        Description:  description,
        Price:        price,
        Quantity:     uint(quantity),
        CategoryName: categoryName,
        Images:       imageURLs,
    }

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
    if quantityStr := c.PostForm("quantity"); quantityStr != "" {
        quantity, err := strconv.ParseUint(quantityStr, 10, 32)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid quantity format"})
            return
        }
        product.Quantity = uint(quantity)
    }
    if categoryName := c.PostForm("categoryName"); categoryName != "" {
        var category adminModels.Category
        if err := database.DB.Where("category_name = ?", categoryName).First(&category).Error; err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
            return
        }
        if !category.Status {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": fmt.Sprintf("Category '%s' is unlisted and cannot be used", categoryName),
            })
            return
        }
        product.CategoryName = categoryName
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
