package helper

import (
	"fmt"
	"os"

	"gopkg.in/gomail.v2"
)

func SendOTP(email, otp string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("EMAIL_USER")) 
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Verification Code for Signup")
	m.SetBody("text", "Your OTP for signup is: "+otp)

	d := gomail.NewDialer("smtp.gmail.com", 587, os.Getenv("EMAIL_USER"), os.Getenv("EMAIL_PASS"))

	if err := d.DialAndSend(m); err != nil {
		fmt.Println("Error:", err)
		return err
	}
	return nil
}
