package helper

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

func GenerateOTP() string {
	pkg.Log.Debug("Generating OTP")

	const otpLength = 6
	const digits = "0123456789"

	otp := make([]byte, otpLength)
	for i := 0; i < otpLength; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			fallback := fmt.Sprintf("%06d", i+1)
			pkg.Log.Warn("Failed to generate random number for OTP, using fallback",
				zap.Error(err),
				zap.String("fallbackOTP", fallback))
			return fallback
		}
		otp[i] = digits[num.Int64()]
	}

	pkg.Log.Debug("OTP generated successfully")
	return string(otp)
}

func GenerateAndStoreOTP(userID string) (string, error) {
	pkg.Log.Debug("Generating and storing OTP for user",
		zap.String("userID", userID))

	otp := GenerateOTP()
	expiry := 5 * time.Minute
	otpKey := "otp:" + userID

	ctx := context.Background()
	err := rdb.Set(ctx, otpKey, otp, expiry).Err()
	if err != nil {
		pkg.Log.Error("Failed to store OTP in Redis",
			zap.String("userID", userID),
			zap.String("otpKey", otpKey),
			zap.Duration("expiry", expiry),
			zap.Error(err))
		return "", fmt.Errorf("error storing OTP: %w", err)
	}

	pkg.Log.Info("OTP stored in Redis successfully",
		zap.String("userID", userID),
		zap.String("otpKey", otpKey),
		zap.Duration("expiry", expiry))
	return otp, nil
}