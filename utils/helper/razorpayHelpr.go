package helper

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
)

func GenerateSignature(orderID, paymentID string) string {
	pkg.Log.Debug("Generating payment signature",
		zap.String("orderID", orderID),
		zap.String("paymentID", paymentID))

	secret := os.Getenv("RAZORPAY_KEY_SECRET")
	if secret == "" {
		pkg.Log.Error("RAZORPAY_KEY_SECRET not set in environment",
			zap.String("orderID", orderID),
			zap.String("paymentID", paymentID))
		return ""
	}

	data := orderID + "|" + paymentID
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))

	pkg.Log.Debug("Payment signature generated successfully",
		zap.String("orderID", orderID),
		zap.String("paymentID", paymentID))
	return signature
}

func VerifyPaymentSignature(orderID, paymentID, signature string) bool {
	pkg.Log.Debug("Verifying payment signature",
		zap.String("orderID", orderID),
		zap.String("paymentID", paymentID))

	expectedSignature := GenerateSignature(orderID, paymentID)
	if expectedSignature == "" {
		pkg.Log.Error("Failed to generate expected signature for verification",
			zap.String("orderID", orderID),
			zap.String("paymentID", paymentID))
		return false
	}

	if expectedSignature == signature {
		pkg.Log.Info("Payment signature verified successfully",
			zap.String("orderID", orderID),
			zap.String("paymentID", paymentID))
		return true
	}

	pkg.Log.Warn("Payment signature verification failed",
		zap.String("orderID", orderID),
		zap.String("paymentID", paymentID))
	return false
}