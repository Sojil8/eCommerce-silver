package helper

import (
	"fmt"
	"log"

	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"gorm.io/gorm"
)

func UpdateProductStock(tx *gorm.DB, productID uint) error {
	var products adminModels.Product
	if err := tx.Preload("Variants").First(&products, productID).Error; err != nil {
		return fmt.Errorf("failed to find product %d: %v", productID, err)
	}

	var totalStock uint
	for _, variant := range products.Variants {
		totalStock += variant.Stock
	}

	products.InStock = totalStock > 0

	if err := tx.Save(&products).Error; err != nil {
		return fmt.Errorf("failed to update product %d InStock status: %v", productID, err)
	}
	log.Printf("Updated product %d InStock status: %t (Total stock: %d)", productID, products.InStock, totalStock)
	return nil

}
