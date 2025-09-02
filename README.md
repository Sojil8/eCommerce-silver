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

### 👤 User Side
- User signup/login with **OTP verification**
- Referral code system
- Browse products (watches)
- Cart management (Add/Update/Delete)
- Apply coupons at checkout
- Place orders & view order history
- Download invoices (PDF)

### 🛠️ Admin Side
- Manage products (Add/Update/Delete)
- Coupon management
- Order & return request handling
- Dashboard with analytics & reports

---

## 📂 Project Structure
├── controllers # All route handlers (User & Admin)
├── database # Database connection and setup
├── middleware # JWT authentication middleware
├── models # GORM models (User, Product, Order, etc.)
├── routes # Route definitions
├── utils/helper # Helper functions (OTP, response handlers, etc.)
├── main.go # Entry point
├── go.mod # Go modules
├── go.sum
└── .env # Environment variables
