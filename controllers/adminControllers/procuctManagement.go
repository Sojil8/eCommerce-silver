package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

func ShowAddProductForm(c *gin.Context) {
	var categories []adminModels.Category
	if err := database.DB.Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.HTML(http.StatusOK, "productAdd.html", gin.H{
		"Product":    nil,
		"Categories": categories,
	})
}

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
	if err := database.DB.Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.HTML(http.StatusOK, "productAdd.html", gin.H{
		"Product":    &product,
		"Categories": categories,
	})
}

func AddProduct(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

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

	var category adminModels.Category
	if err := database.DB.Where("category_name = ?", requiredFields["categoryName"]).First(&category).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
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
		ProductName:  requiredFields["productName"],
		Description:  requiredFields["description"],
		Price:        price,
		CategoryName: requiredFields["categoryName"],
        CategoryID: category.ID,
		Images:       imageURLs,
		IsListed:     true,
	}

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

    // Update product fields if provided
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
        product.CategoryName = categoryName
        product.CategoryID = category.ID
    }

    // Handle image updates
    files := form.File["images"]
    if len(files) > 0 {
        if len(files)+len(product.Images) < 3 && len(files) < 3 {
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
        product.Images = imagePaths
    } else {
        if len(product.Images) < 3 {
            c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
            return
        }
    }

    // Handle variant updates
    colors := form.Value["color[]"]
    variantPrices := form.Value["variantPrice[]"]
    variantStocks := form.Value["variantStock[]"]
    if len(colors) > 0 {
        if len(colors) != len(variantPrices) || len(colors) != len(variantStocks) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Mismatch in variant fields (color, price, stock)"})
            return
        }

        // Create a map of existing variants by color for easy lookup
        existingVariants := make(map[string]*adminModels.Variants)
        for i, v := range product.Variants {
            existingVariants[v.Color] = &product.Variants[i]
        }

        // Process each variant from the form
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
          

            // If variant exists, update it in the database; otherwise, create a new one
            if variant, exists := existingVariants[color]; exists {
                // Update existing variant
                if err := database.DB.Model(variant).Updates(map[string]interface{}{
                    "ExtraPrice": extraPrice,
                    "Stock":      uint(stock),
                }).Error; err != nil {
                    c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update variant %s", color)})
                    return
                }
            } else {
                // Create new variant
                newVariant := adminModels.Variants{
                    ProductID:  product.ID,
                    Color:      color,
                    ExtraPrice: extraPrice,
                    Stock:      uint(stock),
                }
                if err := database.DB.Create(&newVariant).Error; err != nil {
                    c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create variant %s", color)})
                    return
                }
                product.Variants = append(product.Variants, newVariant)
            }
        }

        // Delete variants that are no longer in the input
        for color, variant := range existingVariants {
            found := false
            for _, inputColor := range colors {
                if inputColor == color {
                    found = true
                    break
                }
            }
            if !found {
                if err := database.DB.Delete(variant).Error; err != nil {
                    c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete variant %s", color)})
                    return
                }
            }
        }
    } else if len(product.Variants) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "At least one variant is required"})
        return
    }

    // Update stock status based on variants
    product.InStock = false
    for _, variant := range product.Variants {
        if variant.Stock > 0 {
            product.InStock = true
            break
        }
    }

    // Save the product
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