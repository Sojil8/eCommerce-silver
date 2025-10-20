package config

import (
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func EnshureWallet(tx *gorm.DB, userID uint) (*userModels.Wallet, error) {
	pkg.Log.Info("Ensuring wallet for user", zap.Uint("user_id", userID))

	var wallet userModels.Wallet
	err := tx.Where("user_id = ?", userID).First(&wallet).Error
	if err == gorm.ErrRecordNotFound {
		pkg.Log.Debug("Wallet not found, creating new wallet", zap.Uint("user_id", userID))
		wallet = userModels.Wallet{
			UserID:  userID,
			Balance: 0.0,
		}
		if err := tx.Create(&wallet).Error; err != nil {
			pkg.Log.Error("Failed to create wallet", zap.Uint("user_id", userID), zap.Error(err))
			return nil, err
		}
		pkg.Log.Info("Wallet created successfully", zap.Uint("user_id", userID), zap.Float64("balance", wallet.Balance))
	} else if err != nil {
		pkg.Log.Error("Failed to query wallet", zap.Uint("user_id", userID), zap.Error(err))
		return nil, err
	} else {
		pkg.Log.Debug("Wallet found", zap.Uint("user_id", userID), zap.Float64("balance", wallet.Balance))
	}

	return &wallet, nil
}
