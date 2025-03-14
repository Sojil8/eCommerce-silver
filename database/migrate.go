package database

import (
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
)

func MigrageHandler() {
	DB.AutoMigrate(
		&userModels.User{}, 
		adminModels.Admin{}, 
		adminModels.Category{}, 
		&adminModels.Product{}, 
		&adminModels.Variants{})
}
