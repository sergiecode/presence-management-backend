package handlers

import (
	"regexp"
	"time"

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

// ValidateYYYYMMDD returns true if the string is exactly YYYY-MM-DD
func ValidateYYYYMMDD(dateStr string) bool {
	// Strict regex: 4 digits, dash, 2 digits, dash, 2 digits
	var re = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if !re.MatchString(dateStr) {
		return false
	}
	_, err := time.Parse("2006-01-02", dateStr)
	return err == nil
}

// ValidateRFC3339 returns true if the string is a valid RFC3339 timestamp
func ValidateRFC3339(ts string) bool {
	_, err := time.Parse(time.RFC3339, ts)
	return err == nil
}
