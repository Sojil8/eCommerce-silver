package routes

import (
	"github.com/Sojil8/eCommerce-silver/config"
	controllers "github.com/Sojil8/eCommerce-silver/controllers/userControllers"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/gin-gonic/gin"
)

func UserRoutes(c *gin.Engine) {
	userGroup := c.Group("/")
	{
		// Existing routes...
		userGroup.GET("/signup", controllers.ShowSignUp)
		userGroup.POST("/signup", controllers.UserSignUp)
		userGroup.GET("/signup/otp", controllers.ShowOTPPage)
		userGroup.POST("/signup/otp", controllers.VerifyOTP)
		userGroup.POST("/signup/otp/resend", controllers.ResendOTP)
		userGroup.GET("/login", controllers.ShowLogin)
		userGroup.POST("/login", controllers.LoginPostUser)

		// New Forgot Password routes
		userGroup.GET("/forgot-password", controllers.ShowForgotPassword)
		userGroup.POST("/forgot-password", controllers.ForgotPasswordRequest)
		userGroup.GET("/forgot-password/otp", controllers.ShowResetOTPPage)
		userGroup.POST("/forgot-password/otp", controllers.VerifyResetOTP)
		userGroup.GET("/forgot-password/reset", controllers.ShowResetPassword)
		userGroup.POST("/forgot-password/reset", controllers.ResetPassword)

		protected := userGroup.Group("")
		protected.Use(middleware.Authenticate("jwt_token", "User", "/login"), middleware.ClearCache())
		{
			protected.GET("/home", controllers.GetUserProducts)
			protected.GET("/product/details/:id", controllers.GetProductDetails)
			protected.GET("/shop", controllers.GetUserShop)
			protected.POST("/shop", controllers.GetUserShop)
			protected.POST("/logout", controllers.LogoutUser)

			//profile routes
			protected.GET("/profile", controllers.ShowProfile)
			protected.GET("/profile/edit", controllers.ShowEditProfile)
			protected.POST("/profile/edit", controllers.EditProfile)
			protected.GET("/profile/verify-email", controllers.ShowVerifyEditEmail)
			protected.POST("/profile/verify-email", controllers.VerifyEditEmail)
			protected.POST("/profile/change-password", controllers.ChangePassword)

			// //wishlist routes
			// protected.GET("/wishlist",controllers.ShowWishlist)
			// protected.POST("/wishlist/add/:id",controllers.AddToWishlist)
			// protected.DELETE("/wishlist/remove/:id",controllers.RemoveWishList)

			//cart comming soon
			protected.GET("/cart", controllers.GetCart)	
			protected.POST("/cart/add", controllers.AddToCart)
			protected.PUT("/cart/update", controllers.UpdateQuantity)
			protected.DELETE("/cart/remove", controllers.RemoveFromCart)

			//address 
            protected.POST("/profile/add-address", controllers.AddAddress)
            protected.POST("/profile/edit-address/:address_id", controllers.EditAddress) 
            protected.POST("/profile/delete-address/:address_id", controllers.DeleteAddress)
			protected.GET("/profile/get-address/:address_id", controllers.GetAddress)

			//checkout
			protected.GET("/checkout",controllers.ShowCheckout)
			protected.POST("/profile/set-default-address/:address_id",controllers.SetDefaultAddress)
			protected.POST("/checkout/place-order",controllers.PlaceOrder)
			protected.GET("/order/success",controllers.ShowOrderSuccess)
		}
		authGroup := c.Group("/auth")
		authGroup.GET("/google", config.GoogleLogin)
		authGroup.GET("/google/callback", config.GoogleCallback)
	}
}
