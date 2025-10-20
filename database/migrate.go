package database

import (
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
)

func MigrageHandler() {
	if DB == nil {
		pkg.Log.Fatal("Database connection is nil, cannot proceed with migration")
	}

	pkg.Log.Info("Starting database migration")

	err := DB.AutoMigrate(
		&userModels.Users{},
		&adminModels.Admin{},
		&adminModels.Category{},
		&adminModels.Product{},
		&adminModels.Variants{},
		&userModels.Wishlist{},
		&userModels.Cart{},
		&userModels.CartItem{},
		&userModels.Address{},
		&userModels.Orders{},
		&userModels.OrderItem{},
		&userModels.Return{},
		&userModels.Cancellation{},
		&adminModels.ShippingAddress{},
		&userModels.Wallet{},
		&userModels.WalletTransaction{},
		&adminModels.PaymentDetails{},
		&adminModels.Coupons{},
		&adminModels.ProductOffer{},
		&adminModels.CategoryOffer{},
		&userModels.OrderBackUp{},
		&userModels.Refral{},
	)
	if err != nil {
		pkg.Log.Fatal("Database migration failed",
			zap.Error(err),
			zap.String("context", "AutoMigrate"))
	}

	pkg.Log.Info("Database migration completed successfully",
		zap.Int("model_count", 22))
}