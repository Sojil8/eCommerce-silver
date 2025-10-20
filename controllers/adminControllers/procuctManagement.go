package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ShowAddProductForm(c *gin.Context) {
	pkg.Log.Info("Handling request to show add product form")

	var categories []adminModels.Category
	if err := database.DB.Find(&categories).Error; err != nil {
		pkg.Log.Error("Failed to fetch categories", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}
	pkg.Log.Debug("Categories loaded", zap.Int("count", len(categories)))

	pkg.Log.Info("Rendering productAdd.html", zap.Int("category_count", len(categories)))
	c.HTML(http.StatusOK, "productAdd.html", gin.H{
		"Product":    nil,
		"Categories": categories,
	})
}

func ShowEditProductForm(c *gin.Context) {
	pkg.Log.Info("Handling request to show edit product form")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.Log.Warn("Invalid product ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	pkg.Log.Debug("Parsed product ID", zap.Int("id", id))

	var product adminModels.Product
	if err := database.DB.Preload("Variants").First(&product, id).Error; err != nil {
		pkg.Log.Error("Failed to find product", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	pkg.Log.Debug("Product retrieved", zap.Uint("id", product.ID), zap.String("product_name", product.ProductName))

	var categories []adminModels.Category
	if err := database.DB.Find(&categories).Error; err != nil {
		pkg.Log.Error("Failed to fetch categories", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}
	pkg.Log.Debug("Categories loaded", zap.Int("count", len(categories)))

	pkg.Log.Info("Rendering productAdd.html",
		zap.Uint("product_id", product.ID),
		zap.String("product_name", product.ProductName),
		zap.Int("category_count", len(categories)))
	c.HTML(http.StatusOK, "productAdd.html", gin.H{
		"Product":    &product,
		"Categories": categories,
	})
}

func AddProduct(c *gin.Context) {
	pkg.Log.Info("Handling request to add product")

	form, err := c.MultipartForm()
	if err != nil {
		pkg.Log.Error("Failed to parse form data", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	requiredFields := map[string]string{
		"productName":  c.PostForm("productName"),
		"description":  c.PostForm("description"),
		"price":        c.PostForm("price"),
		"categoryName": c.PostForm("categoryName"),
		"brand":        c.PostForm("brand"),
	}
	for field, value := range requiredFields {
		if value == "" {
			pkg.Log.Warn("Missing required field", zap.String("field", field))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s is required", field)})
			return
		}
	}
	pkg.Log.Debug("Received form data",
		zap.String("product_name", requiredFields["productName"]),
		zap.String("category_name", requiredFields["categoryName"]),
		zap.String("brand", requiredFields["brand"]))

	price, err := strconv.ParseFloat(requiredFields["price"], 64)
	if err != nil || price <= 0 {
		pkg.Log.Warn("Invalid price", zap.String("price", requiredFields["price"]), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format or value"})
		return
	}
	pkg.Log.Debug("Parsed price", zap.Float64("price", price))

	var category adminModels.Category
	if err := database.DB.Where("category_name = ?", requiredFields["categoryName"]).First(&category).Error; err != nil {
		pkg.Log.Error("Invalid category name", zap.String("category_name", requiredFields["categoryName"]), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
		return
	}
	pkg.Log.Debug("Category retrieved", zap.Uint("category_id", category.ID), zap.String("category_name", category.CategoryName))

	files := form.File["images"]
	if len(files) < 3 {
		pkg.Log.Warn("Insufficient images", zap.Int("image_count", len(files)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
		return
	}
	pkg.Log.Debug("Received images", zap.Int("image_count", len(files)))

	var imageURLs []string
	for i, file := range files {
		f, err := file.Open()
		if err != nil {
			pkg.Log.Error("Failed to open image", zap.String("filename", file.Filename), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open image"})
			return
		}
		defer f.Close()
		url, err := helper.ProcessImage(c, f, file)
		if err != nil {
			pkg.Log.Error("Failed to process image", zap.String("filename", file.Filename), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process image"})
			return
		}
		imageURLs = append(imageURLs, url)
		pkg.Log.Debug("Image processed", zap.String("filename", file.Filename), zap.String("url", url), zap.Int("index", i))
	}

	colors := form.Value["color[]"]
	variantPrices := form.Value["variantPrice[]"]
	variantStocks := form.Value["variantStock[]"]
	if len(colors) == 0 {
		pkg.Log.Warn("No variants provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one variant is required"})
		return
	}
	if len(colors) != len(variantPrices) || len(colors) != len(variantStocks) {
		pkg.Log.Warn("Mismatch in variant fields",
			zap.Int("color_count", len(colors)),
			zap.Int("price_count", len(variantPrices)),
			zap.Int("stock_count", len(variantStocks)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mismatch in variant fields (color, price, stock)"})
		return
	}
	pkg.Log.Debug("Received variants", zap.Int("variant_count", len(colors)))

	product := adminModels.Product{
		ProductName:  requiredFields["productName"],
		Description:  requiredFields["description"],
		Brand:        requiredFields["brand"],
		Price:        price,
		CategoryName: requiredFields["categoryName"],
		CategoryID:   category.ID,
		Images:       imageURLs,
		IsListed:     true,
	}

	for i, color := range colors {
		extraPrice, err := strconv.ParseFloat(variantPrices[i], 64)
		if err != nil {
			pkg.Log.Warn("Invalid extra price for variant", zap.String("color", color), zap.String("price", variantPrices[i]), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid extra price for variant %s", color)})
			return
		}
		stock, err := strconv.ParseUint(variantStocks[i], 10, 32)
		if err != nil {
			pkg.Log.Warn("Invalid stock for variant", zap.String("color", color), zap.String("stock", variantStocks[i]), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid stock for variant %s", color)})
			return
		}
		pkg.Log.Debug("Parsed variant",
			zap.String("color", color),
			zap.Float64("extra_price", extraPrice),
			zap.Uint64("stock", stock))
		product.Variants = append(product.Variants, adminModels.Variants{
			Color:      color,
			ExtraPrice: extraPrice,
			Stock:      uint(stock),
		})
	}

	if err := database.DB.Create(&product).Error; err != nil {
		pkg.Log.Error("Failed to create product", zap.String("product_name", product.ProductName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	pkg.Log.Info("Product created successfully",
		zap.Uint("id", product.ID),
		zap.String("product_name", product.ProductName),
		zap.String("category_name", product.CategoryName),
		zap.Int("variant_count", len(product.Variants)))
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Product added successfully",
		"product": product,
	})
}

func EditProduct(c *gin.Context) {
	pkg.Log.Info("Handling request to edit product")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.Log.Warn("Invalid product ID", zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	pkg.Log.Debug("Parsed product ID", zap.Int("id", id))

	var product adminModels.Product
	if err := database.DB.Preload("Variants").First(&product, id).Error; err != nil {
		pkg.Log.Error("Failed to find product", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	pkg.Log.Debug("Product retrieved", zap.Uint("id", product.ID), zap.String("product_name", product.ProductName))

	form, err := c.MultipartForm()
	if err != nil {
		pkg.Log.Error("Failed to parse form data", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	if name := c.PostForm("productName"); name != "" {
		pkg.Log.Debug("Updating product name", zap.String("new_name", name))
		product.ProductName = name
	}
	if desc := c.PostForm("description"); desc != "" {
		pkg.Log.Debug("Updating description", zap.String("new_description", desc))
		product.Description = desc
	}
	if brand := c.PostForm("brand"); brand != "" {
		pkg.Log.Debug("Updating brand", zap.String("new_brand", brand))
		product.Brand = brand
	}
	if priceStr := c.PostForm("price"); priceStr != "" {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil || price <= 0 {
			pkg.Log.Warn("Invalid price", zap.String("price", priceStr), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format or value"})
			return
		}
		pkg.Log.Debug("Updating price", zap.Float64("new_price", price))
		product.Price = price
	}
	if categoryName := c.PostForm("categoryName"); categoryName != "" {
		var category adminModels.Category
		if err := database.DB.Where("category_name = ?", categoryName).First(&category).Error; err != nil {
			pkg.Log.Error("Invalid category name", zap.String("category_name", categoryName), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
			return
		}
		pkg.Log.Debug("Updating category", zap.Uint("category_id", category.ID), zap.String("category_name", categoryName))
		product.CategoryName = categoryName
		product.CategoryID = category.ID
	}

	files := form.File["images"]
	if len(files) > 0 {
		if len(files)+len(product.Images) < 3 && len(files) < 3 {
			pkg.Log.Warn("Insufficient images", zap.Int("new_image_count", len(files)), zap.Int("existing_image_count", len(product.Images)))
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required when updating images"})
			return
		}
		var imagePaths []string
		for i, file := range files {
			openedFile, err := file.Open()
			if err != nil {
				pkg.Log.Error("Failed to open image", zap.String("filename", file.Filename), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open image file"})
				return
			}
			defer openedFile.Close()
			path, err := helper.ProcessImage(c, openedFile, file)
			if err != nil {
				pkg.Log.Error("Failed to process image", zap.String("filename", file.Filename), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process image"})
				return
			}
			imagePaths = append(imagePaths, path)
			pkg.Log.Debug("Image processed", zap.String("filename", file.Filename), zap.String("url", path), zap.Int("index", i))
		}
		pkg.Log.Debug("Updating images", zap.Int("new_image_count", len(imagePaths)))
		product.Images = imagePaths
	} else {
		if len(product.Images) < 3 {
			pkg.Log.Warn("Insufficient existing images", zap.Int("image_count", len(product.Images)))
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
			return
		}
	}

	colors := form.Value["color[]"]
	variantPrices := form.Value["variantPrice[]"]
	variantStocks := form.Value["variantStock[]"]
	if len(colors) > 0 {
		if len(colors) != len(variantPrices) || len(colors) != len(variantStocks) {
			pkg.Log.Warn("Mismatch in variant fields",
				zap.Int("color_count", len(colors)),
				zap.Int("price_count", len(variantPrices)),
				zap.Int("stock_count", len(variantStocks)))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mismatch in variant fields (color, price, stock)"})
			return
		}

		existingVariants := make(map[string]*adminModels.Variants)
		for i, v := range product.Variants {
			existingVariants[v.Color] = &product.Variants[i]
		}

		for i, color := range colors {
			extraPrice, err := strconv.ParseFloat(variantPrices[i], 64)
			if err != nil {
				pkg.Log.Warn("Invalid extra price for variant", zap.String("color", color), zap.String("price", variantPrices[i]), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid extra price for variant %s", color)})
				return
			}
			stock, err := strconv.ParseUint(variantStocks[i], 10, 32)
			if err != nil {
				pkg.Log.Warn("Invalid stock for variant", zap.String("color", color), zap.String("stock", variantStocks[i]), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid stock for variant %s", color)})
				return
			}
			pkg.Log.Debug("Parsed variant update",
				zap.String("color", color),
				zap.Float64("extra_price", extraPrice),
				zap.Uint64("stock", stock))

			if variant, exists := existingVariants[color]; exists {
				if err := database.DB.Model(variant).Updates(map[string]interface{}{
					"ExtraPrice": extraPrice,
					"Stock":      uint(stock),
				}).Error; err != nil {
					pkg.Log.Error("Failed to update variant", zap.String("color", color), zap.Error(err))
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update variant %s", color)})
					return
				}
				pkg.Log.Debug("Variant updated", zap.String("color", color))
			} else {
				newVariant := adminModels.Variants{
					ProductID:  product.ID,
					Color:      color,
					ExtraPrice: extraPrice,
					Stock:      uint(stock),
				}
				if err := database.DB.Create(&newVariant).Error; err != nil {
					pkg.Log.Error("Failed to create variant", zap.String("color", color), zap.Error(err))
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create variant %s", color)})
					return
				}
				pkg.Log.Debug("Variant created", zap.String("color", color))
				product.Variants = append(product.Variants, newVariant)
			}
		}

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
					pkg.Log.Error("Failed to delete variant", zap.String("color", color), zap.Error(err))
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete variant %s", color)})
					return
				}
				pkg.Log.Debug("Variant deleted", zap.String("color", color))
			}
		}
	} else if len(product.Variants) == 0 {
		pkg.Log.Warn("No variants provided and no existing variants")
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one variant is required"})
		return
	}

	product.InStock = false
	for _, variant := range product.Variants {
		if variant.Stock > 0 {
			product.InStock = true
			break
		}
	}
	pkg.Log.Debug("Updated in_stock status", zap.Bool("in_stock", product.InStock))

	if err := database.DB.Save(&product).Error; err != nil {
		pkg.Log.Error("Failed to update product", zap.Uint("id", product.ID), zap.String("product_name", product.ProductName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	pkg.Log.Info("Product updated successfully",
		zap.Uint("id", product.ID),
		zap.String("product_name", product.ProductName),
		zap.String("category_name", product.CategoryName),
		zap.Int("variant_count", len(product.Variants)))
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Product updated successfully",
		"product": product,
	})
}
