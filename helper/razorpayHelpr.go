package helper

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func GenerateSignature(orderID, paymentID string) string {
	secret := os.Getenv("RAZORPAY_KEY_SECRET")
	data := orderID + "|" + paymentID
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func VerifyPaymentSignature(orderID, paymentID, signature string) bool {
	expectedSignature := GenerateSignature(orderID, paymentID)
	return expectedSignature == signature
}
