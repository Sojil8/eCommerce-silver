package services

import (
	"os"

	"github.com/razorpay/razorpay-go"
)

var RazorpayClient *razorpay.Client

func InitRazorPay() {
	testKey:=os.Getenv("RAZORPAY_KEY_ID")
	secretKey:=os.Getenv("RAZORPAY_KEY_SECRET")
	RazorpayClient = razorpay.NewClient(testKey, secretKey)
}
