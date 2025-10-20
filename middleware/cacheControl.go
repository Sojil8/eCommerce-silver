package middleware

import (
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ClearCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		pkg.Log.Debug("Applying ClearCache middleware",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))

		c.Header("Cache-Control", "no-cache, no-store, no-transform, must-revalidate, private, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "Mon, 26 Jul 1997 05:00:00 GMT")
		c.Header("X-Accel-Expires", "0")

		c.Next()
	}
}

func PreventBackButton() gin.HandlerFunc {
	return func(c *gin.Context) {
		pkg.Log.Debug("Applying PreventBackButton middleware",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))

		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("X-Prevent-Back", "true")

		c.Next()
	}
}