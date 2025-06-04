package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RegisterDashboardRoutes(r *gin.RouterGroup) {
	r.GET("/attendance", getAttendanceStats)
	r.GET("/absences", getAbsenceStats)
	r.GET("/users", getUserStats)
	r.PUT("/checkins/:id", updateCheckinForHR)
	r.GET("/audit-logs", getAuditLogs)
}

// @Summary Get attendance statistics
// @Description Returns attendance stats (total check-ins, on-time %, late %) for date range. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/dashboard/attendance [get]
func getAttendanceStats(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	startDate, endDate := parseDateRange(c)
	var total, onTime, late int64
	db.DB.Model(&models.Checkin{}).Where("date >= ? AND date <= ?", startDate, endDate).Count(&total)
	db.DB.Model(&models.Checkin{}).Where("date >= ? AND date <= ? AND late = ?", startDate, endDate, false).Count(&onTime)
	db.DB.Model(&models.Checkin{}).Where("date >= ? AND date <= ? AND late = ?", startDate, endDate, true).Count(&late)
	pct := func(n, d int64) float64 {
		if d == 0 {
			return 0
		} else {
			return float64(n) * 100 / float64(d)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total_checkins": total,
		"on_time":        onTime,
		"late":           late,
		"on_time_pct":    pct(onTime, total),
		"late_pct":       pct(late, total),
	})
}

// @Summary Get absence statistics
// @Description Returns absence stats (total absences, by type) for date range. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/dashboard/absences [get]
func getAbsenceStats(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	startDate, endDate := parseDateRange(c)
	var total, absence, late, medical int64
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ?", startDate, endDate).Count(&total)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, "absence").Count(&absence)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, "late").Count(&late)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, "medical").Count(&medical)
	c.JSON(http.StatusOK, gin.H{
		"total_absences": total,
		"absence":        absence,
		"late":           late,
		"medical":        medical,
	})
}

// @Summary Get user statistics
// @Description Returns user stats (total, by role). HR/admin only.
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/dashboard/users [get]
func getUserStats(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	var total, employees, hr, admin int64
	db.DB.Model(&models.User{}).Count(&total)
	db.DB.Model(&models.User{}).Where("role = ?", "employee").Count(&employees)
	db.DB.Model(&models.User{}).Where("role = ?", "hr").Count(&hr)
	db.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&admin)
	c.JSON(http.StatusOK, gin.H{
		"total_users": total,
		"employees":   employees,
		"hr":          hr,
		"admin":       admin,
	})
}

// @Summary HR/admin update check-in
// @Description HR or admin can update any user's check-in. Audit log is written. JWT with hr/admin role required.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path int true "Check-in ID"
// @Param checkin body models.CheckinRequest true "Check-in data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/dashboard/checkins/{id} [put]
func updateCheckinForHR(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	var req models.CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	id := c.Param("id")
	var checkin models.Checkin
	if err := db.DB.First(&checkin, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Check-in not found"})
		return
	}
	old := checkin // shallow copy for audit
	checkin.Date = req.Date
	checkin.LocationType = req.LocationType
	checkin.LocationDetail = req.LocationDetail
	checkin.GPSLat = req.GPSLat
	checkin.GPSLong = req.GPSLong
	checkin.Notes = req.Notes
	if err := db.DB.Save(&checkin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update check-in", "details": err.Error()})
		return
	}

	oldJSON, _ := json.Marshal(old)
	newJSON, _ := json.Marshal(checkin)
	audit := models.AuditLog{
		UserEmail:  userClaims["email"].(string),
		Action:     "update_checkin",
		EntityID:   checkin.ID,
		EntityType: "checkin",
		OldValue:   string(oldJSON),
		NewValue:   string(newJSON),
		Timestamp:  time.Now(),
	}
	_ = db.DB.Create(&audit).Error // ignore error for now, or handle/log as needed

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

// @Summary Get audit logs
// @Description Returns audit log entries. HR/admin only. Supports filtering by user_email, action, entity_type, entity_id, date. Paginated.
// @Tags dashboard
// @Produce json
// @Param user_email query string false "User email"
// @Param action query string false "Action"
// @Param entity_type query string false "Entity type"
// @Param entity_id query int false "Entity ID"
// @Param date query string false "Date (YYYY-MM-DD)"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20)"
// @Success 200 {object} models.AuditLogListResponse
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /api/dashboard/audit-logs [get]
func getAuditLogs(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(jwt.MapClaims)
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	q := db.DB.Model(&models.AuditLog{})
	if v := c.Query("user_email"); v != "" {
		q = q.Where("user_email = ?", v)
	}
	if v := c.Query("action"); v != "" {
		q = q.Where("action = ?", v)
	}
	if v := c.Query("entity_type"); v != "" {
		q = q.Where("entity_type = ?", v)
	}
	if v := c.Query("entity_id"); v != "" {
		q = q.Where("entity_id = ?", v)
	}
	if v := c.Query("date"); v != "" {
		q = q.Where("DATE(timestamp) = ?", v)
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
	var total int64
	q.Count(&total)
	var logs []models.AuditLog
	offset := (page - 1) * pageSize
	q.Order("timestamp desc").Limit(pageSize).Offset(offset).Find(&logs)
	c.JSON(http.StatusOK, models.AuditLogListResponse{
		Logs:     logs,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	})
}

// parseDateRange parses startDate/endDate query params, defaults to last 30 days
func parseDateRange(c *gin.Context) (string, string) {
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	if v := c.Query("startDate"); v != "" {
		start = v
	}
	if v := c.Query("endDate"); v != "" {
		end = v
	}
	return start, end
}
