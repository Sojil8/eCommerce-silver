package config

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func VerifiRefralCode(newUserID uint, refralCode string) error {
	if refralCode == "" {
		pkg.Log.Debug("Referral code is empty, skipping verification", zap.Uint("new_user_id", newUserID))
		return nil
	}

	pkg.Log.Info("Verifying referral code", zap.Uint("new_user_id", newUserID), zap.String("referral_code", refralCode))

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var currentUser userModels.Users
		if err := tx.Where("id = ?", newUserID).First(&currentUser).Error; err != nil {
			pkg.Log.Error("Failed to find current user", zap.Uint("user_id", newUserID), zap.Error(err))
			return fmt.Errorf("current user not found: %w", err)
		}
		pkg.Log.Debug("Current user retrieved", zap.Uint("user_id", newUserID), zap.String("username", currentUser.UserName))

		if currentUser.ReferralToken == refralCode {
			pkg.Log.Warn("User attempted to use their own referral code", zap.Uint("user_id", newUserID), zap.String("referral_code", refralCode))
			return fmt.Errorf("can't use user's own code")
		}

		var existingRefral userModels.Refral
		if err := tx.Where("user_id = ?", newUserID).First(&existingRefral).Error; err == nil {
			pkg.Log.Warn("User has already used a referral code", zap.Uint("user_id", newUserID), zap.String("used_code", existingRefral.RefralCode))
			return fmt.Errorf("user %d has already used a referral code", newUserID)
		} else if err != gorm.ErrRecordNotFound {
			pkg.Log.Error("Database error checking referral usage", zap.Uint("user_id", newUserID), zap.Error(err))
			return fmt.Errorf("database error checking referral usage: %w", err)
		}

		var refredUser userModels.Users
		if err := tx.Where("referral_token = ?", refralCode).First(&refredUser).Error; err != nil {
			pkg.Log.Error("Invalid referral code", zap.String("referral_code", refralCode), zap.Error(err))
			return fmt.Errorf("invalid code: %w", err)
		}
		pkg.Log.Debug("Referring user retrieved", zap.Uint("referred_user_id", refredUser.ID), zap.String("username", refredUser.UserName))

		if currentUser.CreatedAt.Before(refredUser.CreatedAt) || currentUser.CreatedAt.Equal(refredUser.CreatedAt) {
			pkg.Log.Warn("Invalid referral: referring user joined after or at the same time as current user",
				zap.Uint("new_user_id", newUserID),
				zap.Uint("referred_user_id", refredUser.ID),
				zap.Time("new_user_created_at", currentUser.CreatedAt),
				zap.Time("referred_user_created_at", refredUser.CreatedAt))
			return fmt.Errorf("cannot use this referral code because the code owner joined after you")
		}

		wallet, err := EnshureWallet(tx, newUserID)
		if err != nil {
			pkg.Log.Error("Failed to ensure wallet for new user", zap.Uint("user_id", newUserID), zap.Error(err))
			return err
		}
		pkg.Log.Debug("Wallet ensured for new user", zap.Uint("user_id", newUserID), zap.Float64("current_balance", wallet.Balance))

		currentBalance := wallet.Balance
		wallet.Balance += 200
		if err := tx.Save(&wallet).Error; err != nil {
			pkg.Log.Error("Failed to update wallet balance for new user", zap.Uint("user_id", newUserID), zap.Error(err))
			return fmt.Errorf("error giving reward to user: %w", err)
		}
		pkg.Log.Info("New user wallet updated with referral reward",
			zap.Uint("user_id", newUserID),
			zap.Float64("new_balance", wallet.Balance),
			zap.Float64("reward_amount", 200))

		walletTransaction := userModels.WalletTransaction{
			UserID:        newUserID,
			WalletID:      wallet.ID,
			Amount:        200,
			LastBalance:   currentBalance,
			Description:   fmt.Sprintf("Referral reward for signing up with code %s", refralCode),
			Type:          "Credited",
			Receipt:       "rcpt-" + uuid.New().String(),
			OrderID:       "", // No order associated with referral
			TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
			PaymentMethod: "Referral",
		}
		if err := tx.Create(&walletTransaction).Error; err != nil {
			pkg.Log.Error("Failed to create wallet transaction for new user referral reward",
				zap.Uint("user_id", newUserID),
				zap.String("transaction_id", walletTransaction.TransactionID),
				zap.Error(err))
			return fmt.Errorf("failed to create wallet transaction: %w", err)
		}
		pkg.Log.Info("Wallet transaction created for new user",
			zap.Uint("user_id", newUserID),
			zap.String("transaction_id", walletTransaction.TransactionID))

		var invitePersonWallet userModels.Wallet
		if err := tx.Where("user_id = ?", refredUser.ID).First(&invitePersonWallet).Error; err != nil {
			pkg.Log.Error("Failed to find wallet for referring user", zap.Uint("referred_user_id", refredUser.ID), zap.Error(err))
			return fmt.Errorf("user wallet not found: %w", err)
		}
		pkg.Log.Debug("Wallet retrieved for referring user", zap.Uint("referred_user_id", refredUser.ID), zap.Float64("current_balance", invitePersonWallet.Balance))

		currentBalanceInvite := invitePersonWallet.Balance
		invitePersonWallet.Balance += 100
		if err := tx.Save(&invitePersonWallet).Error; err != nil {
			pkg.Log.Error("Failed to update wallet balance for referring user", zap.Uint("referred_user_id", refredUser.ID), zap.Error(err))
			return fmt.Errorf("error giving reward to user: %w", err)
		}
		pkg.Log.Info("Referring user wallet updated with referral reward",
			zap.Uint("referred_user_id", refredUser.ID),
			zap.Float64("new_balance", invitePersonWallet.Balance),
			zap.Float64("reward_amount", 100))

		walletTransactionInvite := userModels.WalletTransaction{
			UserID:        refredUser.ID,
			WalletID:      invitePersonWallet.ID,
			Amount:        100,
			LastBalance:   currentBalanceInvite,
			Description:   fmt.Sprintf("Referral reward for inviting user %d with code %s", newUserID, refralCode),
			Type:          "Credited",
			Receipt:       "rcpt-" + uuid.New().String(),
			OrderID:       "", // No order associated with referral
			TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
			PaymentMethod: "Referral",
		}
		if err := tx.Create(&walletTransactionInvite).Error; err != nil {
			pkg.Log.Error("Failed to create wallet transaction for referring user reward",
				zap.Uint("referred_user_id", refredUser.ID),
				zap.String("transaction_id", walletTransactionInvite.TransactionID),
				zap.Error(err))
			return fmt.Errorf("failed to create wallet transaction: %w", err)
		}
		pkg.Log.Info("Wallet transaction created for referring user",
			zap.Uint("referred_user_id", refredUser.ID),
			zap.String("transaction_id", walletTransactionInvite.TransactionID))

		createUserRefral := userModels.Refral{
			UserID:             uint(newUserID),
			RefredUserID:       refredUser.ID,
			RewardUser:         200,
			RewardInvitePerson: 100,
			RefralCode:         refralCode,
			ReferralIsUsed:     true,
		}
		if err := tx.Create(&createUserRefral).Error; err != nil {
			pkg.Log.Error("Failed to create referral record",
				zap.Uint("new_user_id", newUserID),
				zap.Uint("referred_user_id", refredUser.ID),
				zap.String("referral_code", refralCode),
				zap.Error(err))
			return fmt.Errorf("referral data create error: %w", err)
		}
		pkg.Log.Info("Referral record created successfully",
			zap.Uint("new_user_id", newUserID),
			zap.Uint("referred_user_id", refredUser.ID),
			zap.String("referral_code", refralCode))

		return nil
	})
}