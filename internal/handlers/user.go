package handlers

import (
	"net/http"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(r *gin.RouterGroup) {
	r.GET("/me", getMe)
	r.GET("/:id", getUserByID)
	r.PUT("/:id", updateUser)
	r.DELETE("/:id", deleteUser)
	r.PUT("/:id/checkin-config", updateUserCheckinConfig)
}

// getMe godoc
// @Summary Get current user's profile
// @Description Returns the authenticated user's profile info
// @Tags users
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {object} gin.H
// @Router /api/users/me [get]
func getMe(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	email, ok := userClaims["email"].(string)
	if !ok {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}
	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	c.JSON(200, user)
}

func getUserByID(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "get user by id (stub)"})
}

func updateUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "update user (stub)"})
}

func deleteUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "delete user (stub)"})
}

// @Summary Update user's check-in config
// @Description HR/admin can set per-user check-in start time and timezone. JWT with hr/admin required.
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param config body models.CheckinConfigRequest true "Check-in config"
// @Success 200 {object} models.User
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/users/{id}/checkin-config [put]
func updateUserCheckinConfig(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	id := c.Param("id")
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	var req models.CheckinConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if req.CheckinStartTime != "" {
		user.CheckinStartTime = req.CheckinStartTime
	}
	if req.Timezone != "" {
		user.Timezone = req.Timezone
	}
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update user", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}
