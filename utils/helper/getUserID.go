package helper

import (
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func FetchUserID(c *gin.Context) uint {
	pkg.Log.Debug("Fetching user ID from context",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	userID, exists := c.Get("id")
	if !exists {
		pkg.Log.Warn("User ID not found in context",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))
		return 0
	}

	uid, ok := userID.(uint)
	if !ok {
		pkg.Log.Warn("Invalid user ID type in context",
			zap.Any("userID", userID),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))
		return 0
	}

	pkg.Log.Debug("User ID fetched successfully",
		zap.Uint("userID", uid),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))
	return uid
}