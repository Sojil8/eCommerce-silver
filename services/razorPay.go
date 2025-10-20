package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/razorpay/razorpay-go"
	"go.uber.org/zap"
)

var RazorpayClient *razorpay.Client

func InitRazorPay() {
	pkg.Log.Debug("Initializing Razorpay client")

	testKey := os.Getenv("RAZORPAY_KEY_ID")
	secretKey := os.Getenv("RAZORPAY_KEY_SECRET")
	if testKey == "" || secretKey == "" {
		pkg.Log.Fatal("Failed to initialize Razorpay client: missing key ID or secret",
			zap.String("keyID", testKey),
			zap.String("secretKey", secretKey))
	}

	RazorpayClient = razorpay.NewClient(testKey, secretKey)
	pkg.Log.Info("Razorpay client initialized successfully")
}

func CreateRazorpayOrder(amountPaise int) (map[string]interface{}, error) {
	pkg.Log.Debug("Creating Razorpay order",
		zap.Int("amountPaise", amountPaise))

	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
	rand.Seed(time.Now().UnixNano())
	receipt := fmt.Sprintf("r-%d-%d", time.Now().UnixNano(), rand.Intn(10000))

	data := map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  receipt,
	}

	order, err := client.Order.Create(data, nil)
	if err != nil {
		pkg.Log.Error("Failed to create Razorpay order",
			zap.String("receipt", receipt),
			zap.Int("amountPaise", amountPaise),
			zap.Error(err))
		return nil, err
	}

	pkg.Log.Info("Razorpay order created successfully",
		zap.String("orderID", order["id"].(string)),
		zap.String("receipt", receipt),
		zap.Int("amountPaise", amountPaise))
	return order, nil
}

func VerifyRazorpaySignature(orderID, paymentID, signature string) error {
	pkg.Log.Debug("Verifying Razorpay signature",
		zap.String("orderID", orderID),
		zap.String("paymentID", paymentID))

	payload := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(os.Getenv("RAZORPAY_KEY_SECRET")))
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if expectedSignature != signature {
		pkg.Log.Error("Razorpay signature verification failed",
			zap.String("orderID", orderID),
			zap.String("paymentID", paymentID),
			zap.String("expectedSignature", expectedSignature),
			zap.String("providedSignature", signature))
		return fmt.Errorf("invalid signature: expected %s, got %s", expectedSignature, signature)
	}

	pkg.Log.Info("Razorpay signature verified successfully",
		zap.String("orderID", orderID),
		zap.String("paymentID", paymentID))
	return nil
}