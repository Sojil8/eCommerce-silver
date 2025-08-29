package database

import (
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
)

func MigrageHandler() {
	DB.AutoMigrate(
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
}
