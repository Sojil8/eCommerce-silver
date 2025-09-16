package routes

import (
	controllers "github.com/Sojil8/eCommerce-silver/controllers/userControllers"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/gin-gonic/gin"
)

func UserRoutes(c *gin.Engine) {
	userGroup := c.Group("/")
	{
		userGroup.Use(middleware.ClearCache())
		userGroup.GET("/signup", controllers.ShowSignUp)
		userGroup.POST("/signup", controllers.UserSignUp)
		userGroup.GET("/signup/otp", controllers.ShowOTPPage)
		userGroup.POST("/signup/otp", controllers.VerifyOTP)
		userGroup.POST("/signup/otp/resend", controllers.ResendOTP)
		userGroup.GET("/login", controllers.ShowLogin)
		userGroup.POST("/login", controllers.LoginPostUser)

		//Forgot Password routes
		userGroup.GET("/forgot-password", controllers.ShowForgotPassword)
		userGroup.POST("/forgot-password", controllers.ForgotPasswordRequest)
		userGroup.GET("/forgot-password/otp", controllers.ShowResetOTPPage)
		userGroup.POST("/forgot-password/otp", controllers.VerifyResetOTP)
		userGroup.GET("/forgot-password/reset", controllers.ShowResetPassword)
		userGroup.POST("/forgot-password/reset", controllers.ResetPassword)

		authGroup := c.Group("/auth")
		authGroup.GET("/google", services.GoogleLogin)
		authGroup.GET("/google/callback", services.GoogleCallback)

		protected := userGroup.Group("")
		protected.Use(middleware.Authenticate("jwt_token", "User", "/login"), middleware.ClearCache())
		{
			protected.GET("/home", controllers.GetUserProducts)
			protected.GET("/product/details/:id", controllers.GetProductDetails)
			protected.GET("/shop", controllers.GetUserShop)
			protected.POST("/shop", controllers.GetUserShop)

			// Profile
			protected.GET("/profile", controllers.ShowProfile)
			protected.GET("/profile/edit", controllers.ShowEditProfile)
			protected.POST("/profile/edit", controllers.EditProfile)
			protected.GET("/profile/verify-email", controllers.ShowVerifyEditEmail)
			protected.POST("/profile/verify-email", controllers.VerifyEditEmail)
			protected.GET("/profile/change-password", controllers.ShowChangePassword)
			protected.POST("/profile/change-password", controllers.ChangePassword)
			protected.GET("/wallet", controllers.ShowWallet)
			protected.GET("/refral", controllers.ShowRefralPage)
			// protected.GET("/referral-data", controllers.GetReferralData)
			// protected.POST("/refral-invite", controllers.VerifiRefralCode)

			// Cart
			protected.GET("/cart", controllers.GetCart)
			protected.POST("/cart/add", controllers.AddToCart)
			protected.PUT("/cart/update", controllers.UpdateQuantity)
			protected.DELETE("/cart/remove", controllers.RemoveFromCart)

			// Address
			protected.GET("/profile/add-address", controllers.GetAddressProfile)
			protected.POST("/profile/add-address", controllers.AddAddress)
			protected.POST("/profile/edit-address/:address_id", controllers.EditAddress)
			protected.POST("/profile/delete-address/:address_id", controllers.DeleteAddress)
			protected.GET("/profile/get-address/:address_id", controllers.GetEditAddress)

			// Checkout
			protected.GET("/checkout", controllers.ShowCheckout)
			protected.POST("/profile/set-default-address/:address_id", controllers.SetDefaultAddress)
			protected.POST("/checkout/place-order", controllers.PlaceOrder)
			// protected.POST("/checkout/create-razorpay-order",controllers.CreateRazorpayOrder)
			protected.POST("/checkout/verify-payment", controllers.VerifyPayment)
			protected.GET("/order/success", controllers.ShowOrderSuccess)
			protected.GET("/order/failure", controllers.ShowOrderFailure)

			// Order
			protected.GET("/orders", controllers.GetOrderList)
			protected.POST("/orders/cancel/:order_id", controllers.CancelOrder)
			protected.POST("/orders/cancel-item/:order_id/:item_id", controllers.CancelOrderItem)
			protected.POST("/orders/return/:order_id", controllers.ReturnOrder)
			protected.GET("/orders/details/:order_id", controllers.ShowOrderDetails)
			protected.GET("/orders/invoice/:order_id", controllers.DownloadInvoice)
			protected.GET("/orders/search", controllers.GetOrderList)
			protected.POST("/orders/retry-payment/:order_id", controllers.RetryPayment) 

			//wishlist
			protected.GET("/wishlist", controllers.ShowWishlist)
			protected.POST("/wishlist/add/:id", controllers.AddToWishlist)
			protected.DELETE("/wishlist/remove/:id", controllers.RemoveWishList)

			//coupons
			protected.POST("/checkout/apply-coupon", controllers.ApplyCoupon)
			protected.POST("/checkout/remove-coupon", controllers.RemoveCoupon)
			protected.GET("/checkout/available-coupons", controllers.GetAvailableCoupons)

			protected.POST("/logout", controllers.LogoutUser)
		}	

		c.NoRoute(controllers.NotFound)
	}
}

