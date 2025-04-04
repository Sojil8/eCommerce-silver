package helper

import "os"

func VerifyPaymentSignature(params map[string]interface{},signature string)bool{
	secret:=os.Getenv("RAZORPAY_KEY_SECRET")
	orderID:=params
}