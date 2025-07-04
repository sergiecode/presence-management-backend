package handlers

import (
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

func RegisterCheckinRoutes(r *gin.RouterGroup) {
	r.POST("/", submitCheckin)
	r.GET("/", getCheckinHistory)
	r.GET("/all", listAllCheckins)
	r.GET("/today", getTodayCheckin)
	r.PUT("/:id", updateCheckin)
	r.DELETE("/:id", deleteCheckin)
	r.POST("/checkout", submitCheckout)
	r.PUT("/checkout/:id", updateCheckoutForHR)
	r.POST("/batch-approve", batchApproveCheckins)
}

// @Summary Submit daily check-in
// @Description User submits daily check-in with location and optional GPS. Only one per day. JWT required. If late, must provide reason.
// @Tags checkin
// @Accept json
// @Produce json
// @Param checkin body models.CheckinRequest true "Check-in data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/checkins [post]
func submitCheckin(c *gin.Context) {
	log.Println("submitCheckin called")

	var req models.CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	claims, ok := c.Get("user")
	if !ok {
		log.Println("No JWT claims found")
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	userID, _ := userClaims["user_id"].(float64)

	// Fetch user for check-in config
	var user models.User
	if err := db.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "User not found"})
		return
	}

	// Determine check-in time (UTC now or provided)
	now := time.Now().UTC()
	var checkinDT time.Time
	if req.Time != "" {
		t, err := time.Parse(time.RFC3339, req.Time)
		if err == nil {
			checkinDT = t
		} else {
			checkinDateTimeStr := req.Date + "T" + req.Time
			loc, err := time.LoadLocation(user.Timezone)
			if err != nil {
				loc = time.UTC
			}
			checkinDT, err = time.ParseInLocation("2006-01-02T15:04:05", checkinDateTimeStr, loc)
			if err != nil {
				c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid check-in time"})
				return
			}
		}
	} else {
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

	dateStr := checkinDT.In(loc).Format("2006-01-02")
	thresholdDT, err := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+startTime, loc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Invalid check-in threshold config"})
		return
	}

	late := false
	if checkinDT.After(thresholdDT) {
		late = true
		if req.LateReason == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Late check-in requires a reason"})
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
				Type:   "late",
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

	// Save checkin (create or update)
	if checkin.ID == 0 {
		if err := db.DB.Create(&checkin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create check-in", Details: err.Error()})
			return
		}
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
		AbsenceID:      checkin.AbsenceID,
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get user's check-in history
// @Description Returns all check-ins for the authenticated user, newest first. JWT required.
// @Tags checkin
// @Produce json
// @Success 200 {array} models.CheckinResponse
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
	var checkins []models.Checkin
	err := db.DB.Where("user_id = ?", uint(userID)).Order("date desc").Find(&checkins).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch check-ins", Details: err.Error()})
		return
	}
	resp := make([]models.CheckinResponse, len(checkins))
	for i, ch := range checkins {
		var checkoutTime *string
		if ch.CheckoutTime != nil {
			ts := ch.CheckoutTime.Format(time.RFC3339)
			checkoutTime = &ts
		}
		resp[i] = models.CheckinResponse{
			ID:             ch.ID,
			UserID:         ch.UserID,
			Date:           ch.Date,
			Time:           ch.Time.Format(time.RFC3339),
			LocationType:   ch.LocationType,
			LocationDetail: ch.LocationDetail,
			GPSLat:         ch.GPSLat,
			GPSLong:        ch.GPSLong,
			Notes:          ch.Notes,
			Late:           ch.Late,
			LateReason:     ch.LateReason,
			CreatedAt:      ch.CreatedAt.Format(time.RFC3339),
			CheckoutTime:   checkoutTime,
			CheckoutStatus: ch.CheckoutStatus,
			Overtime:       ch.Overtime,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get today's check-in
// @Description Returns today's check-in for the authenticated user, or 404 if none. JWT required.
// @Tags checkin
// @Produce json
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
	err := db.DB.Where("user_id = ? AND date = ?", uint(userID), today).First(&checkin).Error
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "No check-in for today"})
		return
	}
	var checkoutTime *string
	if checkin.CheckoutTime != nil {
		ts := checkin.CheckoutTime.Format(time.RFC3339)
		checkoutTime = &ts
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
		CheckoutTime:   checkoutTime,
		CheckoutStatus: checkin.CheckoutStatus,
		Overtime:       checkin.Overtime,
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
		q = q.Where("date = ?", v)
	}
	q = q.Order("date desc").Offset((page - 1) * pageSize).Limit(pageSize)
	q.Find(&checkins)
	resp := make([]models.CheckinResponse, len(checkins))
	for i, ch := range checkins {
		var checkoutTime *string
		if ch.CheckoutTime != nil {
			ts := ch.CheckoutTime.Format(time.RFC3339)
			checkoutTime = &ts
		}
		resp[i] = models.CheckinResponse{
			ID:             ch.ID,
			UserID:         ch.UserID,
			Date:           ch.Date,
			Time:           ch.Time.Format(time.RFC3339),
			LocationType:   ch.LocationType,
			LocationDetail: ch.LocationDetail,
			GPSLat:         ch.GPSLat,
			GPSLong:        ch.GPSLong,
			Notes:          ch.Notes,
			Late:           ch.Late,
			LateReason:     ch.LateReason,
			CreatedAt:      ch.CreatedAt.Format(time.RFC3339),
			CheckoutTime:   checkoutTime,
			CheckoutStatus: ch.CheckoutStatus,
			Overtime:       ch.Overtime,
		}
	}
	c.JSON(200, resp)
}

// @Summary Soft delete check-in (HR/admin)
// @Description HR/admin only. Soft delete a check-in by setting deleted=true.
// @Tags checkin
// @Produce json
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
// @Param checkout body models.CheckoutRequest true "Checkout data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/checkins/checkout [post]
func submitCheckout(c *gin.Context) {
	var req models.CheckoutRequest
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
	userID, _ := userClaims["user_id"].(float64)
	// Find today's check-in
	today := time.Now().Format("2006-01-02")
	var checkin models.Checkin
	err := db.DB.Where("user_id = ? AND date = ?", uint(userID), today).First(&checkin).Error
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No check-in found for today. You must check in before checking out."})
		return
	}
	if checkin.CheckoutTime != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Already checked out for today."})
		return
	}
	now := time.Now().UTC()
	if now.Before(checkin.Time) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Checkout time cannot be before check-in time."})
		return
	}
	checkin.CheckoutTime = &now
	checkin.CheckoutStatus = req.Status
	checkin.Overtime = req.Overtime
	if err := db.DB.Save(&checkin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to record checkout", Details: err.Error()})
		return
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

// @Summary Admin/HR update checkout
// @Description HR or admin can update any user's checkout info (time, status, overtime). JWT with hr/admin role required.
// @Tags checkin
// @Accept json
// @Produce json
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
	var checkoutTime *string
	if checkin.CheckoutTime != nil {
		ts := checkin.CheckoutTime.Format(time.RFC3339)
		checkoutTime = &ts
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
		CheckoutTime:   checkoutTime,
		CheckoutStatus: checkin.CheckoutStatus,
		Overtime:       checkin.Overtime,
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Batch approve check-ins (HR/admin)
// @Description HR/admin can approve or reject multiple check-ins in one API call.
// @Tags checkin
// @Accept json
// @Produce json
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
