package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func GetUserEmail(c *gin.Context) string {
	claims, ok := c.Get("user")
	if !ok {
		return ""
	}
	userClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	email, _ := userClaims["email"].(string)
	return email
}
