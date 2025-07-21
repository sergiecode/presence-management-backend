package handlers

import (
	"fmt"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/logger"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// USER ROUTES - User profile and management
func RegisterUserRoutes(r *gin.RouterGroup) {
	// Current user operations
	r.GET("/me", getMe)
	r.PATCH("/me", updateMyProfile)                    // PATCH for partial updates
	r.PUT("/me/checkin-config", updateMyCheckinConfig) // Users can update their own config

	// Admin user management
	r.GET("/", listUsers)        // GET /api/users
	r.GET("/:id", getUserByID)   // GET /api/users/:id
	r.PUT("/:id", updateUser)    // PUT /api/users/:id (full update)
	r.DELETE("/:id", deleteUser) // DELETE /api/users/:id

	// Admin user configuration - support both PUT and PATCH
	r.PUT("/:id/checkin-config", updateUserCheckinConfig)
	r.PATCH("/:id/checkin-config", updateUserCheckinConfig)
	r.PUT("/:id/hr-details", updateUserHRDetails)

	// Admin user status management
	r.PUT("/:id/approve", approveUser)
	r.PUT("/:id/activate-email", activateUserEmail)
}

// getMe godoc
// @Summary Get current user's profile
// @Description Returns the authenticated user's profile info
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.User
// @Failure 401 {object} models.ErrorResponse
// @Router /api/users/me [get]
func getMe(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	email, ok := userClaims["email"].(string)
	if !ok {
		c.JSON(401, models.ErrorResponse{Error: "Invalid token"})
		return
	}
	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		logger.Log.Error("User not found",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", email),
			zap.Error(err),
		)
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	c.JSON(200, user)
}

// @Summary List all users
// @Description Admin only. Returns paginated list of users. Query params: page, page_size, team, role, on_site_required
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Param team query string false "Filter by team"
// @Param role query string false "Filter by role"
// @Param on_site_required query bool false "Filter by on-site requirement"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
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

	query := db.DB.Model(&models.User{}).Where("active = ? OR pending_approval = ?", true, true)

	// Apply filters
	if team := c.Query("team"); team != "" {
		query = query.Where("team = ?", team)
	}
	if roleFilter := c.Query("role"); roleFilter != "" {
		query = query.Where("role = ?", roleFilter)
	}
	if onSiteRequired := c.Query("on_site_required"); onSiteRequired != "" {
		query = query.Where("on_site_required = ?", onSiteRequired == "true")
	}

	// Get total count
	var total int64
	query.Count(&total)

	var users []models.User
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)

	responses := make([]models.UserResponse, len(users))
	for i, u := range users {
		responses[i] = models.ToUserResponse(u)
	}

	c.JSON(200, gin.H{
		"data": responses,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// @Summary Get user by ID
// @Description Admin only. Get user details by ID.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} models.User
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
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
	if err := db.DB.First(&user, id).Error; err != nil || !user.Active {
		logger.Log.Error("User not found or inactive",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Error(err),
		)
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	resp := models.ToUserResponse(user)
	c.JSON(200, resp)
}

// @Summary Update user
// @Description Admin only. Update user details by ID.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param user body models.User true "User data"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
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
	if err := db.DB.First(&user, id).Error; err != nil || !user.Active {
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
	// Handle new HR fields
	if req.DNI != "" {
		user.DNI = req.DNI
	}
	if req.CUIL != "" {
		user.CUIL = req.CUIL
	}
	if req.BirthDate != nil {
		user.BirthDate = req.BirthDate
	}
	if req.HireDate != nil {
		user.HireDate = req.HireDate
	}
	if req.Location != (models.Location{}) {
		user.Location = req.Location
	}
	if req.WeeklyHours > 0 {
		user.WeeklyHours = req.WeeklyHours
	}
	if req.Notes != "" {
		user.Notes = req.Notes
	}
	if req.Team != "" {
		user.Team = req.Team
	}
	user.ZohoAccess = req.ZohoAccess
	user.TeamsAccess = req.TeamsAccess
	user.OnSiteRequired = req.OnSiteRequired
	if req.WeeklyObjectiveDays > 0 {
		user.WeeklyObjectiveDays = req.WeeklyObjectiveDays
	}
	if req.MonthlyObjectiveDays > 0 {
		user.MonthlyObjectiveDays = req.MonthlyObjectiveDays
	}
	if req.OfficeDays != "" {
		user.OfficeDays = req.OfficeDays
	}
	if err := db.DB.Save(&user).Error; err != nil {
		logger.Log.Error("Failed to update user",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(500, gin.H{"error": "Failed to update user", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}

// @Summary Deactivate (soft delete) user
// @Description Admin only. Soft delete user by setting deactivated=true.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} models.SimpleResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
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
	if err := db.DB.First(&user, id).Error; err != nil || !user.Active {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	user.Active = false
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
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param config body models.CheckinConfigRequest true "Check-in config"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
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
	if req.CheckoutEndTime != "" {
		user.CheckoutEndTime = req.CheckoutEndTime
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
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} models.SimpleResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
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
	user.Active = true
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to approve user"})
		return
	}
	c.JSON(200, gin.H{"message": "User approved"})
}

// Activate user email (HR/admin only)
// @Summary Activate user email
// @Description HR/admin can set email_confirmed=true for a user
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} models.SimpleResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/users/{id}/activate-email [put]
func activateUserEmail(c *gin.Context) {
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
	user.EmailConfirmed = true
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to activate email"})
		return
	}
	c.JSON(200, gin.H{"message": "User email activated"})
}

// @Summary Update user HR details
// @Description HR/admin can update additional user fields from Excel files (DNI, CUIL, birth date, hire date, location, etc.). JWT with hr/admin required.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param details body models.UserHRDetailsRequest true "HR details"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/users/{id}/hr-details [put]
func updateUserHRDetails(c *gin.Context) {
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
	if err := db.DB.First(&user, id).Error; err != nil || !user.Active {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	var req models.UserHRDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Update fields if provided
	if req.DNI != "" {
		user.DNI = req.DNI
	}
	if req.CUIL != "" {
		user.CUIL = req.CUIL
	}
	if req.BirthDate != nil {
		user.BirthDate = req.BirthDate
	}
	if req.HireDate != nil {
		user.HireDate = req.HireDate
	}
	if req.Location != (models.Location{}) {
		user.Location = req.Location
	}
	if req.WeeklyHours > 0 {
		user.WeeklyHours = req.WeeklyHours
	}
	if req.Notes != "" {
		user.Notes = req.Notes
	}
	if req.Team != "" {
		user.Team = req.Team
	}
	user.ZohoAccess = req.ZohoAccess
	user.TeamsAccess = req.TeamsAccess
	user.OnSiteRequired = req.OnSiteRequired
	if req.WeeklyObjectiveDays > 0 {
		user.WeeklyObjectiveDays = req.WeeklyObjectiveDays
	}
	if req.MonthlyObjectiveDays > 0 {
		user.MonthlyObjectiveDays = req.MonthlyObjectiveDays
	}
	if req.OfficeDays != "" {
		user.OfficeDays = req.OfficeDays
	}

	if err := db.DB.Save(&user).Error; err != nil {
		logger.Log.Error("Failed to update user HR details",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(500, gin.H{"error": "Failed to update user HR details", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}

// PATCH /api/users/me
func updateMyProfile(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	userID, _ := userClaims["user_id"].(float64)

	var user models.User
	if err := db.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Picture               *string `json:"picture"`
		Phone                 *string `json:"phone"`
		Timezone              *string `json:"timezone"`
		CheckinStartTime      *string `json:"checkin_start_time"`
		NotificationOffsetMin *int    `json:"notification_offset_min"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if req.Picture != nil {
		user.Picture = *req.Picture
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Timezone != nil {
		user.Timezone = *req.Timezone
	}
	if req.CheckinStartTime != nil {
		user.CheckinStartTime = *req.CheckinStartTime
	}
	if req.NotificationOffsetMin != nil {
		user.NotificationOffsetMin = *req.NotificationOffsetMin
	}

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update profile", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}

// updateMyCheckinConfig allows users to update their own checkin configuration
func updateMyCheckinConfig(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	userID, _ := userClaims["user_id"].(float64)

	var user models.User
	if err := db.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	var req models.CheckinConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Update fields if provided
	if req.CheckinStartTime != "" {
		user.CheckinStartTime = req.CheckinStartTime
	}
	if req.Timezone != "" {
		user.Timezone = req.Timezone
	}
	if req.NotificationOffsetMin > 0 {
		user.NotificationOffsetMin = req.NotificationOffsetMin
	}
	if req.CheckoutEndTime != "" {
		user.CheckoutEndTime = req.CheckoutEndTime
	}

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update checkin config", "details": err.Error()})
		return
	}
	c.JSON(200, user)
}
