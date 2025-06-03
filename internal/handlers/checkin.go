package handlers

import (
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
)

func RegisterCheckinRoutes(r *gin.RouterGroup) {
	r.POST("/", submitCheckin)
	r.GET("/", getCheckinHistory)
	r.GET("/today", getTodayCheckin)
	r.PUT("/:id", updateCheckin)
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
	var req models.CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, ok := userClaims["user_id"].(float64) // JWT lib returns float64 for numbers
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Fetch user for check-in config
	var user models.User
	if err := db.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	// Determine check-in time (UTC now or provided)
	now := time.Now().UTC()
	checkinTime := now.Format("15:04:05")
	if req.Time != "" {
		checkinTime = req.Time
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
	dateStr := req.Date
	if dateStr == "" {
		dateStr = now.In(loc).Format("2006-01-02")
	}
	checkinDateTimeStr := dateStr + "T" + checkinTime
	checkinDT, err := time.ParseInLocation("2006-01-02T15:04:05", checkinDateTimeStr, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid check-in time"})
		return
	}
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
			Time:           checkinTime,
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
		checkin.Time = checkinTime
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
		Time:           checkin.Time,
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
	userClaims := claims.(map[string]interface{})
	userID, ok := userClaims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
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
	userClaims := claims.(map[string]interface{})
	userID, ok := userClaims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
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
