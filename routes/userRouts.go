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
			protected.GET("/profile",controllers.ShowProfile)
			protected.GET("/profile/edit",controllers.ShowEditProfile)
			protected.POST("/profile/edit",controllers.EditProfile)
			protected.GET("/profile/verify-email",controllers.ShowVerifyEditEmail)
			protected.POST("/profile/verify-email",controllers.VerifyEditEmail)
			protected.POST("/profile/change-password",controllers.ChangePassword)

		}
		authGroup := c.Group("/auth")
		authGroup.GET("/google", config.GoogleLogin)
		authGroup.GET("/google/callback", config.GoogleCallback)
	}
}