package handlers

import (
	"fmt"
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"log"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RegisterCheckinRoutes(r *gin.RouterGroup) {
	r.POST("/", submitCheckin)
	r.GET("/", getCheckinHistory)
	r.GET("/all", listAllCheckins)
	r.GET("/today", getTodayCheckin)
	r.PUT("/:id", updateCheckin)
	r.DELETE("/:id", deleteCheckin)
}

// @Summary Submit daily check-in
// @Description User submits daily check-in with location and optional GPS. Only one per day. JWT required. If late, must provide reason.
// @Tags checkin
// @Accept json
// @Produce json
// @Param checkin body models.CheckinRequest true "Check-in data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Router /api/checkins [post]
func submitCheckin(c *gin.Context) {
	log.Println("submitCheckin called")

	var req models.CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	claims, ok := c.Get("user")
	if !ok {
		log.Println("No JWT claims found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	log.Printf("JWT claims: %+v\n", userClaims)
	_, _ = userClaims["role"].(string)
	userID, _ := userClaims["user_id"].(float64)
	log.Printf("Looking up user with ID: %v", userID)

	// Fetch user for check-in config
	var user models.User
	if err := db.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	// Determine check-in time (UTC now or provided)
	now := time.Now().UTC()
	var checkinDT time.Time
	if req.Time != "" {
		// Try to parse as RFC3339 (full timestamp)
		t, err := time.Parse(time.RFC3339, req.Time)
		if err == nil {
			checkinDT = t
		} else {
			// Fallback: treat as "HH:MM:SS" and combine with date
			checkinDateTimeStr := req.Date + "T" + req.Time
			loc, err := time.LoadLocation(user.Timezone)
			if err != nil {
				loc = time.UTC
			}
			checkinDT, err = time.ParseInLocation("2006-01-02T15:04:05", checkinDateTimeStr, loc)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid check-in time"})
				return
			}
		}
	} else {
		// Default: use now
		checkinDT = now.In(time.UTC)
	}

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
		startTime = "09:00"
	}

	// Parse check-in time in user's local tz
	dateStr := checkinDT.In(loc).Format("2006-01-02")

	// Parse threshold time for that day
	thresholdDT, err := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+startTime, loc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid check-in threshold config"})
		return
	}

	late := false
	if checkinDT.After(thresholdDT) {
		late = true
		if req.LateReason == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Late check-in requires a reason"})
			return
		}
	}

	// Upsert: check if check-in exists for this user/date
	var checkin models.Checkin
	err = db.DB.Where("user_id = ? AND date = ?", uint(userID), dateStr).First(&checkin).Error
	if err != nil {
		// Not found, create new
		checkin = models.Checkin{
			UserID:         uint(userID),
			Date:           dateStr,
			Time:           checkinDT,
			LocationType:   req.LocationType,
			LocationDetail: req.LocationDetail,
			GPSLat:         req.GPSLat,
			GPSLong:        req.GPSLong,
			Notes:          req.Notes,
			Late:           late,
			LateReason:     req.LateReason,
		}
		err = db.DB.Create(&checkin).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create check-in", "details": err.Error()})
			return
		}
	} else {
		// Exists, update
		checkin.Time = checkinDT
		checkin.LocationType = req.LocationType
		checkin.LocationDetail = req.LocationDetail
		checkin.GPSLat = req.GPSLat
		checkin.GPSLong = req.GPSLong
		checkin.Notes = req.Notes
		checkin.Late = late
		checkin.LateReason = req.LateReason
		err = db.DB.Save(&checkin).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update check-in", "details": err.Error()})
			return
		}
	}
	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Date:           checkin.Date,
		Time:           checkin.Time.Format(time.RFC3339),
		LocationType:   checkin.LocationType,
		LocationDetail: checkin.LocationDetail,
		GPSLat:         checkin.GPSLat,
		GPSLong:        checkin.GPSLong,
		Notes:          checkin.Notes,
		Late:           checkin.Late,
		LateReason:     checkin.LateReason,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get user's check-in history
// @Description Returns all check-ins for the authenticated user, newest first. JWT required.
// @Tags checkin
// @Produce json
// @Success 200 {array} models.CheckinResponse
// @Failure 401 {object} gin.H
// @Router /api/checkins [get]
func getCheckinHistory(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	_, _ = userClaims["role"].(string)
	userID, _ := userClaims["user_id"].(float64)
	var checkins []models.Checkin
	err := db.DB.Where("user_id = ?", uint(userID)).Order("date desc").Find(&checkins).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch check-ins", "details": err.Error()})
		return
	}
	resp := make([]models.CheckinResponse, len(checkins))
	for i, ch := range checkins {
		resp[i] = models.CheckinResponse{
			ID:             ch.ID,
			UserID:         ch.UserID,
			Date:           ch.Date,
			LocationType:   ch.LocationType,
			LocationDetail: ch.LocationDetail,
			GPSLat:         ch.GPSLat,
			GPSLong:        ch.GPSLong,
			Notes:          ch.Notes,
			CreatedAt:      ch.CreatedAt.Format(time.RFC3339),
		}
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get today's check-in
// @Description Returns today's check-in for the authenticated user, or 404 if none. JWT required.
// @Tags checkin
// @Produce json
// @Success 200 {object} models.CheckinResponse
// @Failure 401 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/checkins/today [get]
func getTodayCheckin(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	_, _ = userClaims["role"].(string)
	userID, _ := userClaims["user_id"].(float64)
	today := time.Now().Format("2006-01-02")
	var checkin models.Checkin
	err := db.DB.Where("user_id = ? AND date = ?", uint(userID), today).First(&checkin).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No check-in for today"})
		return
	}
	resp := models.CheckinResponse{
		ID:             checkin.ID,
		UserID:         checkin.UserID,
		Date:           checkin.Date,
		LocationType:   checkin.LocationType,
		LocationDetail: checkin.LocationDetail,
		GPSLat:         checkin.GPSLat,
		GPSLong:        checkin.GPSLong,
		Notes:          checkin.Notes,
		CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Not allowed
// @Description Users cannot update check-ins. Use HR endpoint.
// @Tags checkin
// @Router /api/checkins/{id} [put]
// @Failure 405 {object} gin.H
func updateCheckin(c *gin.Context) {
	c.JSON(405, gin.H{"error": "Updating check-ins is not allowed. Contact HR."})
}

// @Summary List all check-ins (HR/admin)
// @Description HR/admin only. Returns paginated list of all check-ins. Query params: page, page_size, user_id, date
// @Tags checkin
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20)"
// @Param user_id query int false "Filter by user ID"
// @Param date query string false "Filter by date (YYYY-MM-DD)"
// @Success 200 {object} []models.CheckinResponse
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/checkins/all [get]
func listAllCheckins(c *gin.Context) {
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
		q = q.Where("date = ?", v)
	}
	q = q.Order("date desc").Offset((page - 1) * pageSize).Limit(pageSize)
	q.Find(&checkins)
	resp := make([]models.CheckinResponse, len(checkins))
	for i, ch := range checkins {
		resp[i] = models.CheckinResponse{
			ID:             ch.ID,
			UserID:         ch.UserID,
			Date:           ch.Date,
			Time:           ch.Time.Format("2006-01-02T15:04:05Z07:00"),
			LocationType:   ch.LocationType,
			LocationDetail: ch.LocationDetail,
			GPSLat:         ch.GPSLat,
			GPSLong:        ch.GPSLong,
			Notes:          ch.Notes,
			Late:           ch.Late,
			LateReason:     ch.LateReason,
			CreatedAt:      ch.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	c.JSON(200, resp)
}

// @Summary Soft delete check-in (HR/admin)
// @Description HR/admin only. Soft delete a check-in by setting deleted=true.
// @Tags checkin
// @Produce json
// @Param id path int true "Check-in ID"
// @Success 200 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/checkins/{id} [delete]
func deleteCheckin(c *gin.Context) {
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
	var checkin models.Checkin
	if err := db.DB.First(&checkin, id).Error; err != nil || checkin.Deleted {
		c.JSON(404, gin.H{"error": "Check-in not found"})
		return
	}
	checkin.Deleted = true
	if err := db.DB.Save(&checkin).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete check-in", "details": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Check-in deleted"})
}
