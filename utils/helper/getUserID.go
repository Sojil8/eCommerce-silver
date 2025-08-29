package helper

import "github.com/gin-gonic/gin"

func FetchUserID(c *gin.Context) uint {
	userID, exists := c.Get("id")
	if !exists {
		return 0
	}
	uid, ok := userID.(uint)
	if !ok {
		return 0
	}
	return uid
}
