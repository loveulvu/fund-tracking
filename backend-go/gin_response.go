package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}

func Fail(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"code":    code,
		"message": message,
	})
}

func ErrorFail(c *gin.Context, status int, code string, message string) {
	c.JSON(status, ErrorResponse{
		Error:   code,
		Message: message,
	})
}

func RequireGinAuthClaims(c *gin.Context) (*AuthClaims, bool) {
	claims, ok := getGinAuthClaims(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return nil, false
	}
	return claims, true
}
