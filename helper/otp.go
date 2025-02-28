package helper

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

// GenerateOTP returns a random 6-digit OTP as a string
func GenerateOTP() string {
	const otpLength = 6
	const digits = "0123456789"

	otp := make([]byte, otpLength)
	for i := 0; i < otpLength; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "123456" // Fallback OTP in case of error
		}
		otp[i] = digits[num.Int64()]
	}

	return string(otp)
}

// Redis client setup
var rdb = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

// Generate and Store OTP in Redis
func GenerateAndStoreOTP(userID string) (string, error) {
	otp := GenerateOTP() // Get OTP as a string
	expiry := 5 * time.Minute

	ctx := context.Background()
	err := rdb.Set(ctx, "otp:"+userID, otp, expiry).Err()
	if err != nil {
		return "", fmt.Errorf("error storing OTP: %v", err)
	}

	return otp, nil
}
