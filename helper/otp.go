package helper

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

func GenerateOTP() string {
	const otpLength = 6
	const digits = "0123456789"

	otp := make([]byte, otpLength)
	for i := 0; i < otpLength; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return fmt.Sprintf("%06d", i+1) 
		}
		otp[i] = digits[num.Int64()]
	}
	return string(otp)
}

func GenerateAndStoreOTP(userID string) (string, error) {
	otp := GenerateOTP()
	expiry := 5 * time.Minute

	ctx := context.Background()
	err := rdb.Set(ctx, "otp:"+userID, otp, expiry).Err()
	if err != nil {
		return "", fmt.Errorf("error storing OTP: %v", err)
	}
	return otp, nil
}