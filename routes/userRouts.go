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
		userGroup.GET("/signup", controllers.ShowSignUp)
		userGroup.POST("/signup", controllers.UserSignUp)
		userGroup.GET("/signup/otp", controllers.ShowOTPPage)
		userGroup.POST("/signup/otp", controllers.VerifyOTP)
		userGroup.GET("/login", controllers.ShowLogin)
		userGroup.POST("/login", controllers.LoginPostUser)

		protected := userGroup.Group("")
		protected.Use(middleware.AuthenticateUser())
		{
			protected.GET("/home", controllers.GetUserProducts)
			protected.GET("/product/details/:id", controllers.GetProductDetails)
			protected.GET("/shop",controllers.GetUserShop)
			protected.POST("/shop",controllers.GetUserShop)

			protected.POST("/logout", controllers.LogoutUser) 
		}
		authGroup:=c.Group("/auth")
		authGroup.GET("/google",config.GoogleLogin)
		authGroup.GET("/google/callback",config.GoogleCallback)
	}
}
