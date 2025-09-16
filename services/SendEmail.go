package services

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/gomail.v2"
)

func SendOTP(email, otp string) error {
	if email == "" || otp == "" {
		return fmt.Errorf("email and OTP cannot be empty")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("No Reply <%s>", os.Getenv("EMAIL_USER")))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "🎉 Welcome! Complete Your Signup")

	m.SetHeader("Message-ID", fmt.Sprintf("<%d@yourdomain.com>", time.Now().Unix()))

	htmlBody := getSignupHTMLTemplate(otp)
	plainBody := getSignupPlainTextTemplate(otp)

	m.SetBody("text/plain", plainBody)
	m.AddAlternative("text/html", htmlBody)

	d := gomail.NewDialer("smtp.gmail.com", 587, os.Getenv("EMAIL_USER"), os.Getenv("EMAIL_PASS"))

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send signup OTP email to %s: %w", email, err)
	}

	return nil
}

func getSignupHTMLTemplate(otp string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome - Complete Your Signup</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f4f4f4;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 0 20px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .logo {
            font-size: 28px;
            font-weight: bold;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .welcome-banner {
            background: linear-gradient(135deg, #56ab2f 0%%, #a8e6cf 100%%);
            color: white;
            text-align: center;
            padding: 25px;
            border-radius: 8px;
            margin-bottom: 25px;
        }
        .otp-container {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            text-align: center;
            padding: 25px;
            border-radius: 8px;
            margin: 25px 0;
        }
        .otp-code {
            font-size: 36px;
            font-weight: bold;
            letter-spacing: 8px;
            margin: 15px 0;
            font-family: 'Courier New', monospace;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        .info-box {
            background-color: #e8f4fd;
            border-left: 4px solid #3498db;
            padding: 15px;
            margin: 20px 0;
            border-radius: 0 5px 5px 0;
        }
        .warning {
            background-color: #fff3cd;
            border: 1px solid #ffeaa7;
            color: #856404;
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            color: #666;
            font-size: 14px;
            border-top: 1px solid #eee;
            padding-top: 20px;
        }
        .steps {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 5px;
            margin: 20px 0;
        }
        .step {
            margin: 10px 0;
            padding-left: 25px;
            position: relative;
        }
        .step::before {
            content: "✓";
            position: absolute;
            left: 0;
            color: #28a745;
            font-weight: bold;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo"></div>
        </div>
        
        <div class="welcome-banner">
            <h1>Welcome to SILVER Ecom	 🎉</h1>
            <p>You're just one step away from getting started</p>
        </div>
        
        <p>Hello and welcome!</p>
        <p>Thank you for signing up with us. We're excited to have you on board! To complete your account setup and ensure the security of your account, please verify your email address.</p>
        
        <div class="otp-container">
            <div style="font-size: 18px; margin-bottom: 10px;">Your Verification Code</div>
            <div class="otp-code">%s</div>
            <div style="font-size: 14px; opacity: 0.9; margin-top: 10px;">⏰ This code expires in 10 minutes</div>
        </div>
        
        <div class="steps">
            <h3 style="margin-top: 0;">Next Steps:</h3>
            <div class="step">Go back to the signup page</div>
            <div class="step">Enter the verification code above</div>
            <div class="step">Complete your account setup</div>
            <div class="step">Start exploring all our features!</div>
        </div>
        
        <div class="info-box">
            <strong>💡 Tip:</strong> Keep this email open while you complete the verification process. You can copy and paste the code directly from here.
        </div>
        
        <div class="warning">
            <strong>🔒 Security Notice:</strong><br>
            • Never share this code with anyone<br>
            • We will never ask you for this code via phone or email<br>
            • If you didn't sign up for an account, you can safely ignore this email
        </div>
        
        <p>If you have any questions or need help getting started, our support team is here to help!</p>
        
        <div class="footer">
            <p><strong>Welcome to the community!</strong> 🌟</p>
            <p>This is an automated message, please do not reply to this email.</p>
            <p>&copy; 2024 Your Company Name. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, otp)
}

func getSignupPlainTextTemplate(otp string) string {
	return fmt.Sprintf(`
🎉 WELCOME TO YOUR APP! 🎉

Hello and welcome!

Thank you for signing up with us. We're excited to have you on board!

To complete your account setup, please verify your email address with the code below:

VERIFICATION CODE: %s

⏰ This code will expire in 10 minutes.

NEXT STEPS:
✓ Go back to the signup page
✓ Enter the verification code above
✓ Complete your account setup
✓ Start exploring all our features!

SECURITY NOTICE:
• Never share this code with anyone
• We will never ask you for this code via phone or email
• If you didn't sign up for an account, you can safely ignore this email

If you have any questions, our support team is here to help!

Welcome to the community! 🌟

---
This is an automated message. Please do not reply to this email.
© 2024 Your Company Name. All rights reserved.
`, otp)
}
