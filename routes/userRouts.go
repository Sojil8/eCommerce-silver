package routes

import (
	controllers "github.com/Sojil8/eCommerce-silver/controllers/userControllers"
	"github.com/gin-gonic/gin"
)

func UserRoutes(c *gin.Engine) {
	userGroup := c.Group("/user")

	userGroup.GET("/signup", controllers.ShowSignUp)
	userGroup.POST("/signup", controllers.UserSignUp)
	userGroup.GET("/signup/otp", controllers.ShowOTPPage)
	userGroup.POST("/signup/otp", controllers.VerifyOTP)
	userGroup.GET("/login", controllers.ShowLogin)
	userGroup.POST("/login", controllers.LoginPostUser)

}
