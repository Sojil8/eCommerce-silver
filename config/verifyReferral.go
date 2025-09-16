package config

import (
	"fmt"
	"log"
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
		return nil
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var currentUser userModels.Users
		if err := tx.Where("id = ?", newUserID).First(&currentUser).Error; err != nil {
			return fmt.Errorf("current user not found %s", err)
		}
		if currentUser.ReferralToken == refralCode {
			return fmt.Errorf("can't use user own code")
		}

		var existingRefral userModels.Refral
		if err := tx.Where("user_id = ?", newUserID).First(&existingRefral).Error; err == nil {
			return fmt.Errorf("user %d has already used a referral code", newUserID)
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("database error checking referral usage: %w", err)
		}

		var refredUser userModels.Users
		if err := tx.Where("referral_token = ?", refralCode).First(&refredUser).Error; err != nil {
			return fmt.Errorf("invalid Code %s", err)
		}

		if currentUser.CreatedAt.Before(refredUser.CreatedAt) || currentUser.CreatedAt.Equal(refredUser.CreatedAt) {
			return fmt.Errorf("you cannot use this referral code because the code owner joined after you")
		}

		wallet, err := EnshureWallet(tx, newUserID)
		if err != nil {
			return err
		}
		currentBalance := wallet.Balance
		wallet.Balance += 200
		if err := tx.Save(&wallet).Error; err != nil {
			return fmt.Errorf("error giving reward to user")
		}

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
			pkg.Log.Error("Failed to create wallet transaction for new user referral reward", zap.Uint("userID", newUserID), zap.Error(err))
			return fmt.Errorf("failed to create wallet transaction: %w", err)
		}

		log.Println("got enshure wallet*************")

		var invitePersonWallet userModels.Wallet
		if err := tx.Where("user_id = ?", refredUser.ID).First(&invitePersonWallet).Error; err != nil {
			return fmt.Errorf("user wallet not found")
		}
		currentBalanceInvite := invitePersonWallet.Balance
		invitePersonWallet.Balance += 100
		if err := tx.Save(&invitePersonWallet).Error; err != nil {
			return fmt.Errorf("error giving reward to user %s", err)
		}

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
			pkg.Log.Error("Failed to create wallet transaction for referring user reward", zap.Uint("userID", refredUser.ID), zap.Error(err))
			return fmt.Errorf("failed to create wallet transaction: %w", err)
		}

		createUserRefral := userModels.Refral{
			UserID:             uint(newUserID),
			RefredUserID:       refredUser.ID,
			RewardUser:         200,
			RewardInvitePerson: 100,
			RefralCode:         refralCode,
			ReferralIsUsed:     true,
		}

		if err := tx.Create(&createUserRefral).Error; err != nil {
			return fmt.Errorf("refral data create error %s", err)
		}

		return nil
	})

}
