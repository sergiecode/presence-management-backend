package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/logger"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

func RegisterDashboardRoutes(r *gin.RouterGroup) {
	r.GET("/attendance", getAttendanceStats)
	r.GET("/absences", getAbsenceStats)
	r.GET("/users", getUserStats)
	r.PUT("/checkins/:id", updateCheckinForHR)
	r.GET("/audit-logs", getAuditLogs)
	r.GET("/checkins/export", exportCheckinsToExcel)
	r.GET("/attendance/getAll", getAttendance)
	r.GET("/daily-summary", getDailySummary)
	r.GET("/checkins/view", getCheckinsView)
	r.GET("/attendance/individual", getIndividualAttendance)
	r.POST("/checkins", createOrReplaceCheckinForHR)
	r.GET("/analytics/monthly", getMonthlyAnalytics)
	r.GET("/analytics/heatmap", getAbsenceHeatmap)
	r.GET("/analytics/overtime", getOvertimeStats)
	r.GET("/analytics/prediction", getLateCheckinPrediction)
}

// @Summary Get attendance statistics
// @Description Returns attendance stats (total check-ins, on-time %, late %) for date range. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
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
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
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
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
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
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
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
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	id := c.Param("id")
	var checkin models.Checkin
	if err := db.DB.First(&checkin, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Check-in not found"})
		return
	}
	old := checkin // shallow copy for audit
	parsedCheckinTime, _ := time.Parse(time.RFC3339, req.Time)
	checkin.Time = parsedCheckinTime // parse req.CheckinTime as time.Time before
	checkin.LocationType = req.LocationType
	checkin.LocationDetail = req.LocationDetail
	checkin.GPSLat = req.GPSLat
	checkin.GPSLong = req.GPSLong
	checkin.Notes = req.Notes
	if err := db.DB.Save(&checkin).Error; err != nil {
		logger.Log.Error("Failed to update check-in (dashboard)",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", userClaims["email"].(string)),
			zap.Any("payload", req),
			zap.Error(err),
		)
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
		Time:           checkin.Time.Format(time.RFC3339),
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
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
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

// @Summary Export check-ins to Excel
// @Description HR/admin only. Export check-in data as Excel file. Filters: startDate, endDate, userId
// @Tags dashboard
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Param userId query int false "User ID"
// @Success 200 {file} file
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/checkins/export [get]
func exportCheckinsToExcel(c *gin.Context) {
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
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	userId := c.Query("userId")
	q := db.DB.Model(&models.Checkin{})
	if startDate != "" {
		q = q.Where("date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("date <= ?", endDate)
	}
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	q = q.Where("deleted = ?", false)
	var checkins []models.Checkin
	q.Order("date desc").Find(&checkins)
	f := excelize.NewFile()
	sheet := "Checkins"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"ID", "UserID", "Date", "Time", "LocationType", "LocationDetail", "GPSLat", "GPSLong", "Notes", "Late", "LateReason"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for row, ch := range checkins {
		// TODO: check valores esperados
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row+2), ch.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row+2), ch.UserID)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row+2), ch.Date)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row+2), ch.Time.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row+2), ch.LocationType)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row+2), ch.LocationDetail)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row+2), ch.GPSLat)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row+2), ch.GPSLong)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row+2), ch.Notes)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", row+2), ch.Late)
		f.SetCellValue(sheet, fmt.Sprintf("K%d", row+2), ch.LateReason)
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=checkins_export.xlsx")
	_ = f.Write(c.Writer)
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

// @Summary Get attendance roll call
// @Description Returns a list of all active employees with their status for a given date. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param date query string true "Date (YYYY-MM-DD)"
// @Success 200 {object} []map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/attendance/rollcall [get]
func getAttendance(c *gin.Context) {
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
	date := c.Query("date")
	if date == "" {
		c.JSON(400, gin.H{"error": "Missing date"})
		return
	}

	type attendanceRow struct {
		UserID uint
		Name   string
		Email  string
		// Checkin fields
		CheckinID        *uint
		CheckinTime      *time.Time
		Late             *bool
		LocationType     *string
		LocationDetail   *string
		GPSLat           *float64
		GPSLong          *float64
		Notes            *string
		LateReason       *string
		CheckinCreatedAt *time.Time
		// Absence fields
		AbsenceID        *uint
		AbsenceType      *string
		AbsenceReason    *string
		FileURL          *string
		AbsenceCreatedAt *time.Time
		// New fields
		CheckoutTime   *time.Time
		CheckoutStatus *string
		Overtime       *bool
	}

	var rows []attendanceRow
	db.DB.Raw(`
	SELECT
	u.id AS user_id,
	u.name,
	u.email,
	c.id AS checkin_id,
	c.time AS checkin_time,
	c.late,
	c.location_type,
	c.location_detail,
	c.gps_lat,
	c.gps_long,
	c.notes,
	c.late_reason,
	c.created_at AS checkin_created_at,
	a.id AS absence_id,
	a.type AS absence_type,
	a.reason AS absence_reason,
	a.file_url,
	a.created_at AS absence_created_at,
	c.checkout_time,
	c.checkout_status,
	c.overtime
	FROM users u
	LEFT JOIN checkins c ON c.user_id = u.id AND c.date = ? AND c.deleted = false
	LEFT JOIN absences a ON a.user_id = u.id AND a.date = ? AND a.deleted = false
	WHERE u.deactivated = false AND u.pending_approval = false
`, date, date).Scan(&rows)

	result := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		status := "absent"
		var checkin *models.CheckinResponse
		var absence *models.AbsenceResponse
		var checkoutTime *time.Time
		if r.CheckinID != nil {
			checkoutTime = nil // Initialize to nil
			if r.CheckoutTime != nil {
				checkoutTime = r.CheckoutTime
			}
			checkin = &models.CheckinResponse{
				ID:             *r.CheckinID,
				UserID:         r.UserID,
				Date:           date,
				Time:           r.CheckinTime.Format("2006-01-02 15:04:05"),
				LocationType:   derefString(r.LocationType),
				LocationDetail: derefString(r.LocationDetail),
				GPSLat:         derefFloat64(r.GPSLat),
				GPSLong:        derefFloat64(r.GPSLong),
				Notes:          derefString(r.Notes),
				Late:           derefBool(r.Late),
				LateReason:     derefString(r.LateReason),
				CreatedAt:      r.CheckinCreatedAt.Format("2006-01-02 15:04:05"),
				CheckoutTime:   checkoutTime,
				CheckoutStatus: derefString(r.CheckoutStatus),
				Overtime:       derefBool(r.Overtime),
			}
			if r.Late != nil && *r.Late {
				status = "late"
			} else {
				status = "present"
			}
		}
		if r.AbsenceID != nil {
			absence = &models.AbsenceResponse{
				ID:        *r.AbsenceID,
				UserID:    r.UserID,
				Date:      date,
				Type:      derefString(r.AbsenceType),
				Reason:    derefString(r.AbsenceReason),
				FileURL:   derefString(r.FileURL),
				CreatedAt: r.AbsenceCreatedAt.Format("2006-01-02 15:04:05"),
			}
			// Optionally, override status if medical
			if r.AbsenceType != nil && *r.AbsenceType == "medical" {
				status = "medical"
			}
		}
		row := map[string]interface{}{
			"user_id": r.UserID,
			"name":    r.Name,
			"email":   r.Email,
			"status":  status,
			"checkin": checkin,
			"absence": absence,
		}
		result = append(result, row)
	}

	tableRows := make([]map[string]interface{}, 0, len(result))
	for _, row := range result {
		checkin := row["checkin"].(*models.CheckinResponse)
		absence := row["absence"].(*models.AbsenceResponse)
		tableRow := map[string]interface{}{
			"user_id":         row["user_id"],
			"name":            row["name"],
			"email":           row["email"],
			"status":          row["status"],
			"checkin_time":    "",
			"location_type":   "",
			"location_detail": "",
			"absence_type":    "",
			"absence_reason":  "",
			"checkout_time":   "",
			"checkout_status": "",
			"overtime":        false,
		}
		if checkin != nil {
			tableRow["checkin_time"] = checkin.Time
			tableRow["location_type"] = checkin.LocationType
			tableRow["location_detail"] = checkin.LocationDetail
			tableRow["checkout_time"] = checkin.CheckoutTime
			tableRow["checkout_status"] = checkin.CheckoutStatus
			tableRow["overtime"] = checkin.Overtime
		}
		if absence != nil {
			tableRow["absence_type"] = absence.Type
			tableRow["absence_reason"] = absence.Reason
		}
		tableRows = append(tableRows, tableRow)
	}
	c.JSON(200, tableRows)
}

// Helper functions for nil deref
func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
func derefFloat64(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0
}
func derefBool(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

// @Summary Get daily summary
// @Description Returns the daily_summary row for a given date. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param date query string true "Date (YYYY-MM-DD)"
// @Success 200 {object} models.DailySummary
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/dashboard/daily-summary [get]
func getDailySummary(c *gin.Context) {
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
	date := c.Query("date")
	if date == "" {
		c.JSON(400, models.ErrorResponse{Error: "Missing date"})
		return
	}
	var summary models.DailySummary
	err := db.DB.Raw("SELECT * FROM daily_summary WHERE date = ?", date).Scan(&summary).Error
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch daily summary", Details: err.Error()})
		return
	}
	if summary.Date == "" {
		c.JSON(404, models.ErrorResponse{Error: "No summary for this date"})
		return
	}
	c.JSON(200, summary)
}

// @Summary Get all checkins for a date (view)
// @Description Returns all checkins for a given date from daily_checkins_view. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param date query string true "Date (YYYY-MM-DD)"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/checkins/view [get]
func getCheckinsView(c *gin.Context) {
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
	date := c.Query("date")
	if date == "" {
		c.JSON(400, models.ErrorResponse{Error: "Missing date"})
		return
	}
	var rows []map[string]interface{}
	err := db.DB.Raw("SELECT * FROM daily_checkins_view WHERE date = ?", date).Scan(&rows).Error
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch checkins view", Details: err.Error()})
		return
	}
	c.JSON(200, rows)
}

// @Summary Get individual attendance (checkin + absence + user)
// @Description Returns checkin, absence, and user info for a given user/date. HR/admin only.
// @Tags dashboard
// @Produce json
// @Param user_id query int true "User ID"
// @Param date query string true "Date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/attendance/individual [get]
func getIndividualAttendance(c *gin.Context) {
	userID := c.Query("user_id")
	date := c.Query("date")
	if userID == "" || date == "" {
		c.JSON(400, models.ErrorResponse{Error: "Missing user_id or date"})
		return
	}
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(404, models.ErrorResponse{Error: "User not found"})
		return
	}
	var checkin models.Checkin
	var absence models.Absence
	checkinErr := db.DB.Where("user_id = ? AND date = ?", userID, date).First(&checkin).Error
	absenceErr := db.DB.Where("user_id = ? AND date = ?", userID, date).First(&absence).Error

	c.JSON(200, gin.H{
		"user":    user,
		"checkin": ifNoRecordReturnNull(checkinErr, checkin),
		"absence": ifNoRecordReturnNull(absenceErr, absence),
	})
}

func ifNoRecordReturnNull(err error, v interface{}) interface{} {
	if err != nil {
		return nil
	}
	return v
}

// @Summary HR/Admin create check-in for any user/date
// @Description Create a check-in for any user/date, even if a soft-deleted one exists. If a deleted check-in exists, restore and update it.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param checkin body models.CheckinRequest true "Check-in data"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/checkins [post]
func createOrReplaceCheckinForHR(c *gin.Context) {
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
	var req models.CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	var checkin models.Checkin
	// Try to find even soft-deleted
	err := db.DB.Unscoped().Where("user_id = ? AND date = ?", req.UserID, req.Date).First(&checkin).Error
	if err == nil {
		// Restore if deleted
		if checkin.Deleted {
			checkin.Deleted = false
		}
		// Update fields
		checkin.Time = parseTimeOrNow(req.Time, req.Date)
		checkin.LocationType = req.LocationType
		checkin.LocationDetail = req.LocationDetail
		checkin.GPSLat = req.GPSLat
		checkin.GPSLong = req.GPSLong
		checkin.Notes = req.Notes
		checkin.Late = req.LateReason != ""
		checkin.LateReason = req.LateReason
		if err := db.DB.Save(&checkin).Error; err != nil {
			logger.Log.Error("Failed to update check-in (dashboard)",
				zap.String("endpoint", c.FullPath()),
				zap.String("method", c.Request.Method),
				zap.String("user", userClaims["email"].(string)),
				zap.Any("payload", req),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update check-in", "details": err.Error()})
			return
		}
	} else {
		// Create new
		checkin = models.Checkin{
			UserID:         req.UserID,
			Date:           req.Date,
			Time:           parseTimeOrNow(req.Time, req.Date),
			LocationType:   req.LocationType,
			LocationDetail: req.LocationDetail,
			GPSLat:         req.GPSLat,
			GPSLong:        req.GPSLong,
			Notes:          req.Notes,
			Late:           req.LateReason != "",
			LateReason:     req.LateReason,
		}
		if err := db.DB.Create(&checkin).Error; err != nil {
			c.JSON(500, models.ErrorResponse{Error: "Failed to create check-in", Details: err.Error()})
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
	c.JSON(200, resp)
}

func parseTimeOrNow(timeStr, dateStr string) time.Time {
	if timeStr != "" {
		// Try RFC3339 with and without timezone
		layouts := []string{
			time.RFC3339,
			"2006-01-02T15:04:05", // no timezone
			"2006-01-02 15:04:05",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, timeStr); err == nil {
				return t
			}
		}
	}
	t, _ := time.Parse("2006-01-02 15:04:05", dateStr+" 09:00:00")
	return t
}

// @Summary Get monthly analytics
// @Description Returns monthly attendance analytics. HR/admin only.
// @Tags dashboard
// @Produce json
// @Success 200 {object} []models.MonthlyStat
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/analytics/monthly [get]
func getMonthlyAnalytics(c *gin.Context) {
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
	var stats []models.MonthlyStat
	db.DB.Raw(`
		SELECT
			to_char(date::date, 'YYYY-MM') AS month,
			COUNT(*) AS total,
			SUM(CASE WHEN late THEN 1 ELSE 0 END) AS late,
			SUM(CASE WHEN overtime THEN 1 ELSE 0 END) AS overtime
		FROM checkins
		WHERE deleted = false
		GROUP BY month
		ORDER BY month DESC
	`).Scan(&stats)
	c.JSON(http.StatusOK, stats)
}

// @Summary Get absence heatmap
// @Description Returns a list of dates with counts of absences. HR/admin only.
// @Tags dashboard
// @Produce json
// @Success 200 {object} []models.AbsenceHeatmapEntry
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/analytics/heatmap [get]
func getAbsenceHeatmap(c *gin.Context) {
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
	var heatmap []models.AbsenceHeatmapEntry
	db.DB.Raw(`
		SELECT date, COUNT(*) AS count
		FROM absences
		WHERE deleted = false
		GROUP BY date
		ORDER BY date
	`).Scan(&heatmap)
	c.JSON(http.StatusOK, heatmap)
}

// @Summary Get overtime stats
// @Description Returns a list of dates with counts of overtime. HR/admin only.
// @Tags dashboard
// @Produce json
// @Success 200 {object} []models.OvertimeStatEntry
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/analytics/overtime [get]
func getOvertimeStats(c *gin.Context) {
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
	var overtime []models.OvertimeStatEntry
	db.DB.Raw(`
		SELECT date, SUM(CASE WHEN overtime THEN 1 ELSE 0 END) AS overtime
		FROM checkins
		WHERE deleted = false
		GROUP BY date
		ORDER BY date
	`).Scan(&overtime)
	c.JSON(http.StatusOK, overtime)
}

// @Summary Get late check-in prediction
// @Description Returns a list of late check-in predictions for the next 7 days. HR/admin only.
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/analytics/prediction [get]
func getLateCheckinPrediction(c *gin.Context) {
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
	var daily []struct {
		Date string
		Late int64
	}
	db.DB.Raw(`
		SELECT date, SUM(CASE WHEN late THEN 1 ELSE 0 END) AS late
		FROM checkins
		WHERE deleted = false
		GROUP BY date
		ORDER BY date DESC
		LIMIT 30
	`).Scan(&daily)

	// Calculate 7-day moving average
	var movingAvg []float64
	for i := 0; i < len(daily)-6; i++ {
		sum := int64(0)
		for j := 0; j < 7; j++ {
			sum += daily[i+j].Late
		}
		movingAvg = append(movingAvg, float64(sum)/7.0)
	}
	c.JSON(200, gin.H{"moving_average": movingAvg})
}
