package config

import (
	"log"

	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"gorm.io/gorm"
)

func EnshureWallet(tx *gorm.DB, userID uint) (*userModels.Wallet, error) {
	var wallet userModels.Wallet
	err := tx.Where("user_id = ?", userID).First(&wallet).Error
	if err == gorm.ErrRecordNotFound {
		// Create new wallet if not found
		wallet = userModels.Wallet{
			UserID:  userID,
			Balance: 0.0,
		}
		if err := tx.Create(&wallet).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	log.Println("in side enshure wallet")
	return &wallet, nil
}
