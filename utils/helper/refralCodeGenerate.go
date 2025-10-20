package helper

import (
	"crypto/rand"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
)

func GenerateReferralCode() (string, error) {
	pkg.Log.Debug("Generating referral code")

	const length = 6
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, length)
	_, err := rand.Read(code)
	if err != nil {
		pkg.Log.Error("Failed to generate random bytes for referral code",
			zap.Error(err))
		return "", err
	}

	for i := 0; i < length; i++ {
		code[i] = charset[int(code[i])%len(charset)]
	}
	referralCode := string(code)

	pkg.Log.Info("Referral code generated successfully",
		zap.String("referralCode", referralCode))
	return referralCode, nil
}