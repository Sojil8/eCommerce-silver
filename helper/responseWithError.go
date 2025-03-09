package helper

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ResponseWithErr(c *gin.Context, status int, err, msg, redirect string) {
	c.JSON(status, gin.H{
		"status":   http.StatusText(status),
		"error":    err,
		"message":  msg,
		"redirect": redirect,
	})
}
