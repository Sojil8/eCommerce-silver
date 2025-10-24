# 🛒 Silver - eCommerce Website

An eCommerce web application built with **Golang (Gin Framework, GORM)** and **PostgreSQL**, featuring both **user-side shopping** and **admin-side management**.  
This project implements essential eCommerce functionality such as product listing, cart management, checkout, orders, coupon system, and admin analytics.

---

## 🚀 Tech Stack
- **Backend:** Go (Gin Framework)
- **Database:** PostgreSQL
- **ORM:** GORM
- **Cache/OTP:** Redis
- **Authentication:** JWT
- **Frontend:** HTML, CSS, JavaScript
- **Payment Integration:** Razorpay

---

## ✨ Features

👤 User Side

🔐 OTP-based signup/login

🤝 Referral code system

🛍️ Browse and filter products (Watches)

🛒 Cart management — Add / Update / Delete items

🎟️ Apply coupons during checkout

💳 Place orders and view order history

📄 Download invoices (PDF)


🛠️ Admin Side

🧩 Product management — Add / Update / Delete products

🎫 Coupon management

📦 Order & Return request handling

📊 Analytics Dashboard — Sales, Revenue, User Activity reports
---


## 📂 Project Structure
├── controllers         # All route handlers (User & Admin)
├── database            # Database connection and setup
├── middleware          # JWT authentication middleware
├── models              # GORM models (User, Product, Order, etc.)
├── routes              # Route definitions
├── utils/helper        # Helper functions (OTP, responses, etc.)
├── templates           # HTML templates (frontend)
├── static              # CSS, JS, and images
├── main.go             # Entry point
├── go.mod              # Go modules
├── go.sum
└── .env                # Environment variables
