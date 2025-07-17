package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/logger"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// CHECKIN ROUTES - Employee check-in/out operations
func RegisterCheckinRoutes(r *gin.RouterGroup) {
	// Primary checkin operations
	r.POST("/", submitCheckin)
	r.POST("/checkout", submitCheckout)
	
	// Individual checkin management
	r.GET("/", getCheckinHistory)               // current user's history
	r.GET("/today", getTodayCheckin)            // current user's today record
	r.PUT("/:id", updateCheckin)
	r.DELETE("/:id", deleteCheckin)
	
	// Admin/HR operations
	r.GET("/all", listAllCheckins)              // all users' checkins (admin)
	r.PUT("/checkout/:id", updateCheckoutForHR) // HR can update checkout
	r.POST("/batch-approve", batchApproveCheckins)
	
	// Location management
	r.PUT("/locations", updateLocations)        // change location during day
}

// @Summary Submit daily check-in
// @Description User submits daily check-in with location. Only one per day. JWT required. If late, must provide reason.
// @Tags checkin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param checkin body models.CheckinRequest true "Check-in data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/checkins [post]
func submitCheckin(c *gin.Context) {
	log.Println("submitCheckin called")

	var req models.CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}

	log.Printf("Checkin request: %+v", req)
	claims, ok := c.Get("user")
	if !ok {
		log.Println("No JWT claims found")
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	userIDFloat, ok := userClaims["user_id"].(float64)
	if !ok {
		log.Printf("Invalid user_id in JWT claims: %v", userClaims["user_id"])
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid user ID in token"})
		return
	}
	userID := uint(userIDFloat)
	log.Printf("Extracted user ID from JWT: %d", userID)

	// Fetch user for check-in config
	var user models.User
	if err := db.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "User not found"})
		return
	}

	// Determine check-in time
	now := time.Now().UTC()
	var checkinDT time.Time

	if req.Time != "" {
		// Try RFC3339 and common formats
		layouts := []string{
			time.RFC3339,
			"2006-01-02T15:04:05", // no timezone
			"2006-01-02 15:04:05",
			"2006-01-02T15:04",
		}
		var err error
		for _, layout := range layouts {
			checkinDT, err = time.Parse(layout, req.Time)
			if err == nil {
				break
			}
		}
		if err != nil {
			// fallback: use now
			checkinDT = now
		}
	} else {
		// fallback: use now
		checkinDT = now
	}

	// Extract date string from checkinDT for absence logic
	dateStr := checkinDT.Format("2006-01-02")

	// Determine user's timezone and threshold
	tz := user.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	startTime := user.CheckinStartTime
	if startTime == "" {
		startTime = "09:00" // Default start time if not configured
	}

	log.Printf("User timezone: %s, check-in start time: %s", tz, startTime)

	thresholdDT, err := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+startTime, loc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Invalid check-in threshold config"})
		return
	}

	// Convert check-in time to user's timezone for comparison
	checkinInUserTZ := checkinDT.In(loc)

	log.Printf("Check-in time: %s, Threshold: %s, Timezone: %s",
		checkinInUserTZ.Format("2006-01-02 15:04:05"),
		thresholdDT.Format("2006-01-02 15:04:05"),
		tz)

	var absenceID *uint
	late := false
	delayMinutes := 0
	if checkinInUserTZ.After(thresholdDT) {
		delay := checkinInUserTZ.Sub(thresholdDT)
		delayMinutes = int(delay.Minutes())
		if delayMinutes > 15 {
			late = true
			if req.LateReason == "" {
				// Provide helpful error message
				configMessage := ""
				if user.CheckinStartTime == "" {
					configMessage = " You can set your check-in time in your profile settings."
				}
				c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error: fmt.Sprintf("Late check-in requires a reason for delays over 15 minutes. You were %d minutes late.%s", delayMinutes, configMessage),
				})
				return
			}
			var absence models.Absence
			absenceErr := db.DB.Where("user_id = ? AND date = ?", user.ID, dateStr).First(&absence).Error
			if absenceErr != nil {
				absence = models.Absence{
					UserID: user.ID,
					Date:   dateStr,
					Type:   models.AbsenceLate,
					Reason: req.LateReason,
				}
				if err := db.DB.Create(&absence).Error; err == nil {
					absenceID = &absence.ID
				}
			} else {
				absenceID = &absence.ID
			}
		}
	}

	// Upsert: check if check-in exists for this user/date
	var checkin models.Checkin
	err = db.DB.Where("user_id = ? AND DATE(time) = ?", uint(userID), dateStr).First(&checkin).Error
	if err != nil {
		// Not found, create new
		checkin = models.Checkin{
			UserID:         uint(userID),
			Time:           checkinDT,
			Notes:          req.Notes,
			Late:           late,
			LateReason:     req.LateReason,
		}
		
		// Create the checkin first
		if err := db.DB.Create(&checkin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create check-in", Details: err.Error()})
			return
		}
	} else {
		// Exists, update
		checkin.Time = checkinDT
		checkin.Notes = req.Notes
		checkin.Late = late
		checkin.LateReason = req.LateReason
		
		// Update the checkin
		if err := db.DB.Save(&checkin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update check-in", Details: err.Error()})
			return
		}
		
		// Delete existing locations
		if err := db.DB.Where("checkin_id = ?", checkin.ID).Delete(&models.CheckinLocation{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to clear existing locations", Details: err.Error()})
			return
		}
	}

	// Add new locations from request
	for _, loc := range req.Locations {
		location := models.CheckinLocation{
			CheckinID:      checkin.ID,
			LocationType:   loc.LocationType,
			LocationDetail: loc.LocationDetail,
		}
		if err := db.DB.Create(&location).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create location", Details: err.Error()})
			return
		}
	}

	// --- Absence integration ---
	if late {
		// Check if absence already exists for this user/date
		var absence models.Absence
		absenceErr := db.DB.Where("user_id = ? AND date = ?", user.ID, dateStr).First(&absence).Error
		if absenceErr != nil {
			// Not found, create absence
			absence = models.Absence{
				UserID: user.ID,
				Date:   dateStr,
				Type:   models.AbsenceLate,
				Reason: "Auto-created from check-in",
			}
			if err := db.DB.Create(&absence).Error; err == nil {
				checkin.AbsenceID = &absence.ID
				// Notify HR/admin (replace with your real HR email) TODO: Replace with HR email
				go models.SendMail("hr@yourcompany.com", "Absence Alert",
					fmt.Sprintf("User %s was late on %s", user.Email, dateStr))
			}
		} else {
			checkin.AbsenceID = &absence.ID
		}
	}

	checkin.AbsenceID = absenceID

	// Save checkin (create or update)
	if checkin.ID == 0 {
		if err := db.DB.Create(&checkin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create check-in", Details: err.Error()})
			return
		}
		oldCheckin := checkin // shallow copy before save
		oldJSON, _ := json.Marshal(oldCheckin)
		newJSON, _ := json.Marshal(checkin)
		audit := models.AuditLog{
			UserEmail:  GetUserEmail(c),
			Action:     "user_checkin",
			EntityID:   checkin.ID,
			EntityType: "checkin",
			OldValue:   string(oldJSON),
			NewValue:   string(newJSON),
			Timestamp:  time.Now(),
		}
		_ = db.DB.Create(&audit).Error // ignore error for now
	} else {
		if err := db.DB.Save(&checkin).Error; err != nil {
			logger.Log.Error("Failed to update check-in",
				zap.String("endpoint", c.FullPath()),
				zap.String("method", c.Request.Method),
				zap.String("user", GetUserEmail(c)),
				zap.Any("payload", req),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update check-in", Details: err.Error()})
			return
		}
	}

	// Load locations for response
	if err := db.DB.Where("checkin_id = ?", checkin.ID).Find(&checkin.Locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to load locations", Details: err.Error()})
		return
	}

	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Time:           checkin.Time.Format(time.RFC3339),
		Notes:          checkin.Notes,
		Late:           checkin.Late,
		LateReason:     checkin.LateReason,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
		AbsenceID:      checkin.AbsenceID,
		Locations:      checkin.Locations,
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get user's check-in history
// @Description Returns all check-ins for the authenticated user, newest first. JWT required.
// @Tags checkin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Router /api/checkins [get]
func getCheckinHistory(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	_, _ = userClaims["role"].(string)
	userID, _ := userClaims["user_id"].(float64)
	
	// Pagination parameters
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
	
	// Get total count
	var total int64
	db.DB.Model(&models.Checkin{}).Where("user_id = ?", uint(userID)).Count(&total)
	
	var checkins []models.Checkin
	err := db.DB.Where("user_id = ?", uint(userID)).
		Order("time desc"). // Changed from date desc to time desc
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&checkins).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch check-ins", Details: err.Error()})
		return
	}
	resp := make([]models.CheckinResponse, len(checkins))
	for i, ch := range checkins {
		var checkoutTime *time.Time
		if ch.CheckoutTime != nil {
			checkoutTime = ch.CheckoutTime
		}
		resp[i] = models.CheckinResponse{
			ID:             ch.ID,
			UserID:         ch.UserID,
			Time:           ch.Time.Format(time.RFC3339),
			Notes:          ch.Notes,
			Late:           ch.Late,
			LateReason:     ch.LateReason,
			CreatedAt:      ch.CreatedAt.Format(time.RFC3339),
			CheckoutTime:   checkoutTime,
			CheckoutStatus: ch.CheckoutStatus,
			Overtime:       ch.Overtime,
			Locations:      ch.Locations,
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"data": resp,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// @Summary Get today's check-in
// @Description Returns today's check-in for the authenticated user, or 404 if none. JWT required.
// @Tags checkin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.CheckinResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/checkins/today [get]
func getTodayCheckin(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	_, _ = userClaims["role"].(string)
	userID, _ := userClaims["user_id"].(float64)
	today := time.Now().Format("2006-01-02")
	var checkin models.Checkin
	err := db.DB.Where("user_id = ? AND DATE(time) = ?", uint(userID), today).First(&checkin).Error
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "No check-in for today"})
		return
	}
	
	// Load locations for response
	if err := db.DB.Where("checkin_id = ?", checkin.ID).Find(&checkin.Locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to load locations", Details: err.Error()})
		return
	}
	
	var checkoutTime *time.Time
	if checkin.CheckoutTime != nil {
		checkoutTime = checkin.CheckoutTime
	}
	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Time:           checkin.Time.Format(time.RFC3339),
		Notes:          checkin.Notes,
		Late:           checkin.Late,
		LateReason:     checkin.LateReason,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
		CheckoutTime:   checkoutTime,
		CheckoutStatus: checkin.CheckoutStatus,
		Overtime:       checkin.Overtime,
		Locations:      checkin.Locations,
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Not allowed
// @Description Users cannot update check-ins. Use HR endpoint.
// @Tags checkin
// @Router /api/checkins/{id} [put]
// @Failure 405 {object} models.ErrorResponse
func updateCheckin(c *gin.Context) {
	c.JSON(405, models.ErrorResponse{Error: "Updating check-ins is not allowed. Contact HR."})
}

// @Summary List all check-ins (HR/admin)
// @Description HR/admin only. Returns paginated list of all check-ins. Query params: page, page_size, user_id, date
// @Tags checkin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20)"
// @Param user_id query int false "Filter by user ID"
// @Param date query string false "Filter by date (YYYY-MM-DD)"
// @Success 200 {object} []models.CheckinResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/checkins/all [get]
func listAllCheckins(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Forbidden: HR or admin only"})
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
	var checkins []models.Checkin
	q := db.DB.Where("deleted = ?", false)
	if v := c.Query("user_id"); v != "" {
		q = q.Where("user_id = ?", v)
	}
	if v := c.Query("date"); v != "" {
		q = q.Where("time >= ? AND time < ?", v+" 00:00:00", v+" 23:59:59") // Filter by date range for time
	}
	q = q.Order("time desc").Offset((page - 1) * pageSize).Limit(pageSize)
	q.Find(&checkins)
	resp := make([]models.CheckinResponse, len(checkins))
	for i, ch := range checkins {
		var checkoutTime *time.Time
		if ch.CheckoutTime != nil {
			checkoutTime = ch.CheckoutTime
		}
		resp[i] = models.CheckinResponse{
			ID:             ch.ID,
			UserID:         ch.UserID,
			Time:           ch.Time.Format(time.RFC3339),
			Notes:          ch.Notes,
			Late:           ch.Late,
			LateReason:     ch.LateReason,
			CreatedAt:      ch.CreatedAt.Format(time.RFC3339),
			CheckoutTime:   checkoutTime,
			CheckoutStatus: ch.CheckoutStatus,
			Overtime:       ch.Overtime,
			Locations:      ch.Locations,
		}
	}
	c.JSON(200, resp)
}

// @Summary Soft delete check-in (HR/admin)
// @Description HR/admin only. Soft delete a check-in by setting deleted=true.
// @Tags checkin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Check-in ID"
// @Success 200 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/checkins/{id} [delete]
func deleteCheckin(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Forbidden: HR or admin only"})
		return
	}
	id := c.Param("id")
	var checkin models.Checkin
	if err := db.DB.First(&checkin, id).Error; err != nil || checkin.Deleted {
		c.JSON(404, models.ErrorResponse{Error: "Check-in not found"})
		return
	}
	checkin.Deleted = true
	if err := db.DB.Save(&checkin).Error; err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to delete check-in", Details: err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Check-in deleted"})
}

// @Summary Submit daily checkout
// @Description User submits daily checkout (end-of-day). Only one per day. JWT required. Must have checked in first. Records checkout time, status, and overtime.
// @Tags checkin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param checkout body models.CheckoutRequest true "Checkout data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/checkins/checkout [post]
func submitCheckout(c *gin.Context) {
	logger.Log.Info("submitCheckout called", zap.String("user_agent", c.Request.UserAgent()))
	
	var req models.CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Error("JSON binding failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	
	logger.Log.Info("Checkout request received", 
		zap.String("checkout_time", req.CheckoutTime),
		zap.Bool("overtime", req.Overtime),
		zap.String("status", req.Status))
	
	claims, ok := c.Get("user")
	if !ok {
		logger.Log.Error("No JWT claims found")
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	
	userClaims := claims.(jwt.MapClaims)
	userIDFloat, ok := userClaims["user_id"].(float64)
	if !ok {
		logger.Log.Error("Invalid user_id in JWT claims", zap.Any("user_id", userClaims["user_id"]))
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid user ID in token"})
		return
	}
	userID := uint(userIDFloat)
	
	logger.Log.Info("Processing checkout for user", zap.Uint("user_id", userID))
	
	// Get user details
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		logger.Log.Error("User not found", zap.Uint("user_id", userID), zap.Error(err))
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
		return
	}
	
	logger.Log.Info("User found", zap.String("email", user.Email), zap.String("timezone", user.Timezone))
	
	// Find the latest check-in for the user that doesn't have a checkout yet
	var checkin models.Checkin
	err := db.DB.Where("user_id = ? AND checkout_time IS NULL", userID).Order("time desc").First(&checkin).Error
	if err != nil {
		logger.Log.Error("No check-in found for checkout", zap.Uint("user_id", userID), zap.Error(err))
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No check-in found for today. You must check in before checking out."})
		return
	}
	
	logger.Log.Info("Check-in found", zap.Uint("checkin_id", checkin.ID), zap.Time("checkin_time", checkin.Time))

	var checkoutTime time.Time
	if req.CheckoutTime != "" {
		// Try multiple time formats
		layouts := []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"15:04",
			"2006-01-02 15:04:05",
		}
		
		var parseErr error
		for _, layout := range layouts {
			if layout == "15:04" {
				// For time-only format, combine with today's date
				today := time.Now().Format("2006-01-02")
				checkoutTime, parseErr = time.Parse("2006-01-02 15:04", today+" "+req.CheckoutTime)
			} else {
				checkoutTime, parseErr = time.Parse(layout, req.CheckoutTime)
			}
			if parseErr == nil {
				logger.Log.Info("Time parsed successfully", zap.String("layout", layout), zap.Time("checkout_time", checkoutTime))
				break
			}
		}
		
		if parseErr != nil {
			logger.Log.Error("Failed to parse checkout time", zap.String("checkout_time", req.CheckoutTime), zap.Error(parseErr))
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid checkout_time format. Use HH:MM, RFC3339, or YYYY-MM-DDTHH:MM:SS"})
			return
		}
	} else {
		checkoutTime = time.Now().UTC()
		logger.Log.Info("Using current time for checkout", zap.Time("checkout_time", checkoutTime))
	}

	// Validate checkout time is not before check-in time
	if checkoutTime.Before(checkin.Time) {
		logger.Log.Error("Checkout time before check-in time", 
			zap.Time("checkout_time", checkoutTime), 
			zap.Time("checkin_time", checkin.Time))
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Checkout time cannot be before check-in time."})
		return
	}

	// Check if it's early checkout
	endTime := user.CheckoutEndTime
	if endTime == "" {
		endTime = "17:00"
	}
	
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil || user.Timezone == "" {
		loc = time.UTC
		logger.Log.Info("Using UTC timezone", zap.String("user_timezone", user.Timezone))
	} else {
		logger.Log.Info("Using user timezone", zap.String("timezone", user.Timezone))
	}
	
	expectedEnd, _ := time.ParseInLocation("2006-01-02T15:04", checkoutTime.Format("2006-01-02")+"T"+endTime, loc)
	logger.Log.Info("Expected end time", zap.Time("expected_end", expectedEnd), zap.String("end_time", endTime))
	
	if checkoutTime.Before(expectedEnd) {
		if req.Status == "" {
			logger.Log.Error("Early checkout without status")
			c.JSON(400, models.ErrorResponse{Error: "Early checkout requires a status/reason"})
			return
		}
		checkin.CheckoutStatus = req.Status
		logger.Log.Info("Early checkout with status", zap.String("status", req.Status))
	}

	checkin.CheckoutTime = &checkoutTime
	checkin.Overtime = req.Overtime
	
	logger.Log.Info("Saving checkout", 
		zap.Time("checkout_time", checkoutTime),
		zap.Bool("overtime", req.Overtime),
		zap.String("status", checkin.CheckoutStatus))
	
	if err := db.DB.Save(&checkin).Error; err != nil {
		logger.Log.Error("Failed to record checkout",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", GetUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to record checkout", Details: err.Error()})
		return
	}
	
	logger.Log.Info("Checkout saved successfully", zap.Uint("checkin_id", checkin.ID))
	
	// Create audit log
	oldCheckin := checkin // shallow copy before save
	oldJSON, _ := json.Marshal(oldCheckin)
	newJSON, _ := json.Marshal(checkin)
	audit := models.AuditLog{
		UserEmail:  GetUserEmail(c),
		Action:     "user_checkout",
		EntityID:   checkin.ID,
		EntityType: "checkin",
		OldValue:   string(oldJSON),
		NewValue:   string(newJSON),
		Timestamp:  time.Now(),
	}
	if auditErr := db.DB.Create(&audit).Error; auditErr != nil {
		logger.Log.Error("Failed to create audit log", zap.Error(auditErr))
		// Don't fail the checkout for audit log errors
	}
	
	// Load locations for the response
	if locErr := db.DB.Model(&checkin).Association("Locations").Find(&checkin.Locations); locErr != nil {
		logger.Log.Error("Failed to load locations", zap.Error(locErr))
		// Don't fail the checkout for location loading errors
	}
	
	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Time:           checkin.Time.Format(time.RFC3339),
		Notes:          checkin.Notes,
		Late:           checkin.Late,
		LateReason:     checkin.LateReason,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
		CheckoutTime:   checkin.CheckoutTime,
		CheckoutStatus: checkin.CheckoutStatus,
		Overtime:       checkin.Overtime,
		Locations:      checkin.Locations,
	}
	
	logger.Log.Info("Checkout completed successfully", zap.Uint("checkin_id", checkin.ID))
	c.JSON(http.StatusOK, resp)
}

// @Summary Admin/HR update checkout
// @Description HR or admin can update any user's checkout info (time, status, overtime). JWT with hr/admin role required.
// @Tags checkin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Check-in ID"
// @Param checkout body models.CheckoutUpdateRequest true "Checkout update data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/checkins/checkout/{id} [put]
func updateCheckoutForHR(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "Forbidden: HR or admin only"})
		return
	}
	id := c.Param("id")
	var req models.CheckoutUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	var checkin models.Checkin
	if err := db.DB.First(&checkin, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Check-in not found"})
		return
	}
	if req.CheckoutTime != "" {
		ts, err := time.Parse(time.RFC3339, req.CheckoutTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid checkout_time format (must be RFC3339)"})
			return
		}
		checkin.CheckoutTime = &ts
	}
	if req.CheckoutStatus != "" {
		checkin.CheckoutStatus = req.CheckoutStatus
	}
	checkin.Overtime = req.Overtime
	if checkin.CheckoutTime != nil && checkin.CheckoutTime.Before(checkin.Time) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Checkout time cannot be before check-in time."})
		return
	}
	if err := db.DB.Save(&checkin).Error; err != nil {
		logger.Log.Error("Failed to update checkout",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", GetUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update checkout", Details: err.Error()})
		return
	}
	var checkoutTime *time.Time
	if checkin.CheckoutTime != nil {
		checkoutTime = checkin.CheckoutTime
	}
	// Load locations for the response
	db.DB.Model(&checkin).Association("Locations").Find(&checkin.Locations)
	
	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Time:           checkin.Time.Format(time.RFC3339),
		Notes:          checkin.Notes,
		Late:           checkin.Late,
		LateReason:     checkin.LateReason,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
		CheckoutTime:   checkoutTime,
		CheckoutStatus: checkin.CheckoutStatus,
		Overtime:       checkin.Overtime,
		Locations:      checkin.Locations,
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Batch approve check-ins (HR/admin)
// @Description HR/admin can approve or reject multiple check-ins in one API call.
// @Tags checkin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body models.BatchApproveRequest true "Batch approve request"
// @Success 200 {object} models.BatchApproveResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/checkins/batch-approve [post]
func batchApproveCheckins(c *gin.Context) {
	type reqBody struct {
		IDs    []uint `json:"ids"`
		Action string `json:"action"` // "approve" or "reject"
		Reason string `json:"reason"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || (req.Action != "approve" && req.Action != "reject") {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	// userEmail, _ := userClaims["email"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}

	tx := db.DB.Begin()
	var processed, failed int
	for _, id := range req.IDs {
		var ch models.Checkin
		if err := tx.First(&ch, id).Error; err != nil || ch.Deleted {
			failed++
			continue
		}
		// You can add more status fields if needed, for now just log action
		ch.Notes = req.Reason
		if err := tx.Save(&ch).Error; err != nil {
			failed++
			continue
		}
		// Optionally: add audit log here
		processed++
	}
	if failed > 0 {
		tx.Rollback()
		c.JSON(500, gin.H{"success": false, "processed": processed, "failed": failed, "message": "Some items failed, transaction rolled back"})
		return
	}
	tx.Commit()
	c.JSON(200, gin.H{"success": true, "processed": processed, "failed": failed, "message": fmt.Sprintf("Processed %d check-ins", processed)})
}

// @Summary Update locations during the day
// @Description User can update their work locations during the day. Only works if they have already checked in today. JWT required.
// @Tags checkin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param locations body models.UpdateLocationsRequest true "Updated locations"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/checkins/locations [put]
func updateLocations(c *gin.Context) {
	var req models.UpdateLocationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}

	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	userIDFloat, ok := userClaims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid user ID in token"})
		return
	}
	userID := uint(userIDFloat)

	// Find today's check-in
	today := time.Now().Format("2006-01-02")
	var checkin models.Checkin
	err := db.DB.Where("user_id = ? AND DATE(time) = ?", userID, today).First(&checkin).Error
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "No check-in found for today. You must check in first."})
		return
	}

	// Delete existing locations
	if err := db.DB.Where("checkin_id = ?", checkin.ID).Delete(&models.CheckinLocation{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to clear existing locations", Details: err.Error()})
		return
	}

	// Add new locations
	for _, loc := range req.Locations {
		location := models.CheckinLocation{
			CheckinID:      checkin.ID,
			LocationType:   loc.LocationType,
			LocationDetail: loc.LocationDetail,
		}
		if err := db.DB.Create(&location).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create location", Details: err.Error()})
			return
		}
	}

	// Load updated locations for response
	if err := db.DB.Where("checkin_id = ?", checkin.ID).Find(&checkin.Locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to load locations", Details: err.Error()})
		return
	}

	var checkoutTime *time.Time
	if checkin.CheckoutTime != nil {
		checkoutTime = checkin.CheckoutTime
	}

	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Time:           checkin.Time.Format(time.RFC3339),
		Notes:          checkin.Notes,
		Late:           checkin.Late,
		LateReason:     checkin.LateReason,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
		CheckoutTime:   checkoutTime,
		CheckoutStatus: checkin.CheckoutStatus,
		Overtime:       checkin.Overtime,
		Locations:      checkin.Locations,
	}
	c.JSON(http.StatusOK, resp)
}
