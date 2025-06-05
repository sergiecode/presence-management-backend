package handlers

import (
	"fmt"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RegisterUserRoutes(r *gin.RouterGroup) {
	r.GET("/me", getMe)
	// Admin-only endpoints
	r.GET("/", listUsers)        // GET /api/users
	r.POST("/", createUser)      // POST /api/users
	r.GET("/:id", getUserByID)   // GET /api/users/:id
	r.PUT("/:id", updateUser)    // PUT /api/users/:id
	r.DELETE("/:id", deleteUser) // DELETE /api/users/:id
	r.PUT("/:id/checkin-config", updateUserCheckinConfig)
	r.PUT("/users/:id/approve", approveUser)
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
	userClaims := claims.(jwt.MapClaims)
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

// @Summary List all users
// @Description Admin only. Returns paginated list of users. Query params: page, page_size
// @Tags users
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20)"
// @Success 200 {object} []models.User
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/users [get]
func listUsers(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: admin only"})
		return
	}
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}
	}
	var users []models.User
	db.DB.Where("deactivated = ?", false).Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)
	c.JSON(200, users)
}

// @Summary Create user
// @Description Admin only. Create a new user. Email must be unique.
// @Tags users
// @Accept json
// @Produce json
// @Param user body models.User true "User data"
// @Success 201 {object} models.User
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/users [post]
func createUser(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: admin only"})
		return
	}
	var req models.User
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if req.Email == "" || req.Name == "" {
		c.JSON(400, gin.H{"error": "Email and name are required"})
		return
	}
	var existing models.User
	if err := db.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(400, gin.H{"error": "User with this email already exists"})
		return
	}
	user := models.User{
		Email:                 req.Email,
		Name:                  req.Name,
		Picture:               req.Picture,
		Role:                  req.Role,
		CheckinStartTime:      req.CheckinStartTime,
		Timezone:              req.Timezone,
		NotificationOffsetMin: req.NotificationOffsetMin,
	}
	if user.Role == "" {
		user.Role = "employee"
	}
	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create user", "details": err.Error()})
		return
	}
	c.JSON(201, user)
}

// @Summary Get user by ID
// @Description Admin only. Get user details by ID.
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} models.User
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/users/{id} [get]
func getUserByID(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: admin only"})
		return
	}
	id := c.Param("id")
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil || user.Deactivated {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	c.JSON(200, user)
}

// @Summary Update user
// @Description Admin only. Update user details by ID.
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param user body models.User true "User data"
// @Success 200 {object} models.User
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/users/{id} [put]
func updateUser(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: admin only"})
		return
	}
	id := c.Param("id")
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil || user.Deactivated {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	var req models.User
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Picture != "" {
		user.Picture = req.Picture
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.CheckinStartTime != "" {
		user.CheckinStartTime = req.CheckinStartTime
	}
	if req.Timezone != "" {
		user.Timezone = req.Timezone
	}
	if req.NotificationOffsetMin > 0 {
		user.NotificationOffsetMin = req.NotificationOffsetMin
	}
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update user", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}

// @Summary Deactivate (soft delete) user
// @Description Admin only. Soft delete user by setting deactivated=true.
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/users/{id} [delete]
func deleteUser(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: admin only"})
		return
	}
	id := c.Param("id")
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil || user.Deactivated {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	user.Deactivated = true
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to deactivate user", "details": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "User deactivated"})
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
	id := c.Param("id")
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	userID, _ := userClaims["user_id"].(float64)

	if role != "hr" && role != "admin" && id != fmt.Sprintf("%.0f", userID) {
		c.JSON(403, gin.H{"error": "Forbidden: can only update your own config"})
		return
	}

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
	if req.NotificationOffsetMin > 0 {
		user.NotificationOffsetMin = req.NotificationOffsetMin
	}
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update user", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}

// @Summary Approve user
// @Description HR/admin can approve a user by setting pending_approval=false and deactivated=false.
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/users/{id}/approve [put]
func approveUser(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
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
	user.PendingApproval = false
	user.Deactivated = false
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to approve user"})
		return
	}
	c.JSON(200, gin.H{"message": "User approved"})
}
