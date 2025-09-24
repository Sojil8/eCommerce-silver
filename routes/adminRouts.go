package routes

import (
	controllers "github.com/Sojil8/eCommerce-silver/controllers/adminControllers"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/gin-gonic/gin"
)

func AdminRoutes(c *gin.Engine) {
	adminGroup := c.Group("/admin")
	{
		adminGroup.GET("/login", controllers.ShowLoginPage)
		adminGroup.POST("/login", controllers.AdminLogin)

		protected := adminGroup.Group("")
		protected.Use(middleware.Authenticate("jwtTokensAdmin", "Admin", "/admin/login"), middleware.ClearCache())
		{
			//user-management route
			protected.GET("/user-management", controllers.GetUsers)
			protected.PATCH("/block-user/:id", controllers.BlockUser)
			protected.PATCH("/unblock-user/:id", controllers.UnBlockUser)

			// Category routes
			protected.GET("/category", controllers.GetCategories)
			protected.POST("/category/add", controllers.AddCategory)
			protected.PATCH("/category/edit/:id", controllers.EditCategory)
			protected.PATCH("/category/list/:id", controllers.ListCategory)
			protected.PATCH("/category/unlist/:id", controllers.UnlistCategory)

			//product roures
			protected.GET("/products", controllers.GetProducts)
			protected.GET("/products/add", controllers.ShowAddProductForm)
			protected.POST("/products/add", controllers.AddProduct)
			protected.GET("/products/edit/:id", controllers.ShowEditProductForm)
			protected.PATCH("/products/edit/:id", controllers.EditProduct)
			protected.GET("/products/details/:id", controllers.ProductDetailsHandler) //product detals admin
			protected.PATCH("/products/toggle/:id", controllers.ToggleProductStatus)

			//order
			protected.GET("/orders", controllers.ListOrder)
			protected.POST("/orders/:order_id/status", controllers.UpdateOrderStatus)
			protected.GET("/orders/:order_id", controllers.ViewOrdetailsAdmin)
			protected.GET("/returns", controllers.ListReturnRequests)
			protected.POST("/returns/:return_id/verify", controllers.VerifyReturnRequest)

			//coupons
			protected.GET("/coupons", controllers.ShowCoupon)
			protected.POST("/coupons/add", controllers.AddCoupon)
			protected.GET("/coupons/get/:id", controllers.GetCoupon)
			protected.POST("/coupons/edit/:id", controllers.EditCoupon)
			protected.DELETE("/coupons/delete/:id", controllers.DeleteCoupon)

			//offers
			protected.GET("/offers", controllers.ShowOfferPage)
			protected.POST("/product_offers/:product_id", controllers.AddProductOffer)
			protected.GET("/product_offers/:id", controllers.ShowEditProductOffer)
			protected.PUT("/product_offers/:id", controllers.EditProductOffer)
			protected.DELETE("/product_offers/:id", controllers.DeleteProductOffer)
			protected.POST("/category_offers/:category_id", controllers.AddCategoryOffer)
			protected.GET("/category_offers/:id", controllers.ShowCategoryOfferEdit)
			protected.PUT("/category_offers/:id", controllers.EditCategoryOffer)
			protected.DELETE("/category_offers/:id", controllers.DeleteCategoryOffer)
			// protected.GET("/apply_offer/:product_id", controllers.ApplyBestOffer)

			//admin dashboard report
			protected.GET("/dashboard", controllers.ShowDashboard)
			protected.GET("/dashboard/data", controllers.GetDashboardData)
			protected.GET("/dashboard/export", controllers.ExportSalesReport)
			protected.POST("/dashboard/log-action", controllers.LogAdminAction)

			protected.POST("/logout", controllers.AdminLogout)
		}
	}
}
