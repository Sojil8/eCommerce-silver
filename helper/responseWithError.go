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

// func ValidateRequesFields(c *gin.Context,field map[string]string,redirect string)bool{
// 	for key,value:=range field{
// 		if value == ""{
// 			ResponseWithErr(c,http.StatusBadRequest,key+"is required","Missing required fields",redirect)
// 			return false
// 		}
// 	}
// 	return true

// }