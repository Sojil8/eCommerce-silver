package helper

import (
	"fmt"

	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func UpdateProductStock(tx *gorm.DB, productID uint) error {
	pkg.Log.Debug("Starting product stock update",
		zap.Uint("productID", productID))

	var products adminModels.Product
	if err := tx.Preload("Variants").First(&products, productID).Error; err != nil {
		pkg.Log.Error("Failed to find product",
			zap.Uint("productID", productID),
			zap.Error(err))
		return fmt.Errorf("failed to find product %d: %w", productID, err)
	}
	pkg.Log.Debug("Product found",
		zap.Uint("productID", productID),
		zap.String("productName", products.ProductName))

	var totalStock uint
	for _, variant := range products.Variants {
		totalStock += variant.Stock
	}
	pkg.Log.Debug("Calculated total stock",
		zap.Uint("productID", productID),
		zap.Uint("totalStock", totalStock))

	products.InStock = totalStock > 0

	if err := tx.Save(&products).Error; err != nil {
		pkg.Log.Error("Failed to update product InStock status",
			zap.Uint("productID", productID),
			zap.Bool("inStock", products.InStock),
			zap.Error(err))
		return fmt.Errorf("failed to update product %d InStock status: %w", productID, err)
	}

	pkg.Log.Info("Updated product InStock status",
		zap.Uint("productID", productID),
		zap.Bool("inStock", products.InStock),
		zap.Uint("totalStock", totalStock))
	return nil
}