package controllers

import (
	"fmt"

	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"gorm.io/gorm"
)

func EnshureWallet(tx *gorm.DB, userId uint) (*userModels.Wallet, error) {
	var wallet userModels.Wallet
	if err := tx.Where("user_id = ?", userId).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			wallet = userModels.Wallet{
				UserID:  userId,
				Balance: 0,
			}
			if err := tx.Create(&wallet).Error; err != nil {
				return nil, fmt.Errorf("failed to create wallet: %v", err)
			}
		}else{
			return nil,fmt.Errorf("failed to fetch wallet: %v", err)
		}
	}

	return &wallet, nil
}
