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

// DASHBOARD ROUTES - Analytics and reporting endpoints
func RegisterDashboardRoutes(r *gin.RouterGroup) {
	// Statistics endpoints
	r.GET("/stats/attendance", getAttendanceStats)
	r.GET("/stats/absences", getAbsenceStats)
	r.GET("/stats/users", getUserStats)
	r.GET("/stats/overtime", getOvertimeStats)

	// Analytics endpoints
	r.GET("/analytics/monthly", getMonthlyAnalytics)
	r.GET("/analytics/heatmap", getAbsenceHeatmap)
	r.GET("/analytics/prediction/late-checkins", getLateCheckinPrediction)

	// Data retrieval endpoints
	r.GET("/attendance/summary", getAttendance) // renamed from getAll
	r.GET("/attendance/individual", getIndividualAttendance)
	r.GET("/attendance/daily-summary", getDailySummary)
	r.GET("/checkins/view", getCheckinsView)
	r.GET("/users/by-team", getUsersByTeam)

	// Export endpoints
	r.GET("/export/checkins", exportCheckinsToExcel)
	r.GET("/export/attendance", exportAttendanceToExcel)

	// Admin management endpoints
	r.PUT("/checkins/:id", updateCheckinForHR)
	r.POST("/checkins", createOrReplaceCheckinForHR)
	r.POST("/convert-absence-to-checkin", convertAbsenceToCheckin) // New endpoint
	r.POST("/create-absence", createAbsenceForHR)                  // New endpoint for HR to create absences
	r.GET("/audit-logs", getAuditLogs)
}

// @Summary Get attendance statistics
// @Description Returns attendance stats (total check-ins, on-time %, late %) for date range. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Security BearerAuth
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/stats/attendance [get]
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
// @Security BearerAuth
// @Security BearerAuth
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/stats/absences [get]
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
	var total, sick, vacation, personal, unauthorized, other int64
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ?", startDate, endDate).Count(&total)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, models.AbsenceSick).Count(&sick)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, models.AbsenceVacation).Count(&vacation)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, models.AbsencePersonal).Count(&personal)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, models.AbsenceUnauthorized).Count(&unauthorized)
	db.DB.Model(&models.Absence{}).Where("date >= ? AND date <= ? AND type = ?", startDate, endDate, models.AbsenceOther).Count(&other)
	c.JSON(http.StatusOK, gin.H{
		"total_absences": total,
		"sick":           sick,
		"vacation":       vacation,
		"personal":       personal,
		"unauthorized":   unauthorized,
		"other":          other,
	})
}

// @Summary Get user statistics
// @Description Returns user stats (total, by role, by team). HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/stats/users [get]
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

	// Get team statistics
	var teamStats []struct {
		Team  string `json:"team"`
		Count int64  `json:"count"`
	}
	db.DB.Model(&models.User{}).
		Select("team, count(*) as count").
		Where("team IS NOT NULL AND team != ''").
		Group("team").
		Scan(&teamStats)

	// Get access statistics
	var zohoCount, teamsCount, onSiteCount int64
	db.DB.Model(&models.User{}).Where("zoho_access = ?", true).Count(&zohoCount)
	db.DB.Model(&models.User{}).Where("teams_access = ?", true).Count(&teamsCount)
	db.DB.Model(&models.User{}).Where("on_site_required = ?", true).Count(&onSiteCount)

	c.JSON(http.StatusOK, gin.H{
		"total_users": total,
		"employees":   employees,
		"hr":          hr,
		"admin":       admin,
		"team_stats":  teamStats,
		"access_stats": gin.H{
			"zoho_access":      zohoCount,
			"teams_access":     teamsCount,
			"on_site_required": onSiteCount,
		},
	})
}

// @Summary HR/admin update check-in
// @Description HR or admin can update any user's check-in. Audit log is written. JWT with hr/admin role required.
// @Tags dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security BearerAuth
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
	checkin.Notes = req.Notes

	// Update locations - first delete existing ones
	db.DB.Where("checkin_id = ?", checkin.ID).Delete(&models.CheckinLocation{})

	// Create new locations
	for _, loc := range req.Locations {
		location := models.CheckinLocation{
			CheckinID:      checkin.ID,
			LocationType:   loc.LocationType,
			LocationDetail: loc.LocationDetail,
		}
		db.DB.Create(&location)
	}
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

	// Load locations for the response
	db.DB.Model(&checkin).Association("Locations").Find(&checkin.Locations)

	resp := models.CheckinResponse{
		ID:        checkin.ID,
		UserID:    checkin.UserID,
		Time:      checkin.Time.Format(time.RFC3339),
		Notes:     checkin.Notes,
		CreatedAt: checkin.CreatedAt.Format(time.RFC3339),
		Locations: checkin.Locations,
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get audit logs
// @Description Returns audit log entries. HR/admin only. Supports filtering by user_email, action, entity_type, entity_id, date. Paginated.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
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

// @Summary Export check-ins to Excel (ART format)
// @Description HR/admin only. Export check-in data as Excel file in ART format with all HR fields. Filters: startDate, endDate, userId
// @Tags dashboard
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Param userId query int false "User ID"
// @Success 200 {file} file
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/export/checkins [get]
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

	// Build query with user join to get HR fields
	query := `
		SELECT 
			u.name || ' ' || u.surname as empleado,
			u.dni,
			u.cuil,
			u.birth_date,
			u.hire_date,
			u.location,
			u.weekly_hours as hrs_semanales,
			u.notes as aclaraciones,
			DATE(c.time) as date,
			c.time,
			cl.location_type,
			cl.location_detail,
			c.late,
			c.late_reason,
			c.checkout_time,
			c.checkout_status,
			c.overtime
		FROM checkins c
		JOIN users u ON c.user_id = u.id
		LEFT JOIN checkin_locations cl ON c.id = cl.checkin_id
		WHERE c.deleted = false
	`

	var args []interface{}
	argCount := 1

	if startDate != "" {
		query += fmt.Sprintf(" AND DATE(c.time) >= $%d", argCount)
		args = append(args, startDate)
		argCount++
	}
	if endDate != "" {
		query += fmt.Sprintf(" AND DATE(c.time) <= $%d", argCount)
		args = append(args, endDate)
		argCount++
	}
	if userId != "" {
		query += fmt.Sprintf(" AND c.user_id = $%d", argCount)
		args = append(args, userId)
		argCount++
	}

	query += " ORDER BY DATE(c.time) DESC, u.name"

	type exportRow struct {
		Empleado       string     `json:"empleado"`
		DNI            string     `json:"dni"`
		CUIL           string     `json:"cuil"`
		BirthDate      *time.Time `json:"birth_date"`
		HireDate       *time.Time `json:"hire_date"`
		Location       string     `json:"location"`
		HrsSemanales   int        `json:"hrs_semanales"`
		Aclaraciones   string     `json:"aclaraciones"`
		Date           string     `json:"date"`
		Time           time.Time  `json:"time"`
		LocationType   int        `json:"location_type"`
		LocationDetail string     `json:"location_detail"`
		Late           bool       `json:"late"`
		LateReason     string     `json:"late_reason"`
		CheckoutTime   *time.Time `json:"checkout_time"`
		CheckoutStatus string     `json:"checkout_status"`
		Overtime       bool       `json:"overtime"`
	}

	var rows []exportRow
	if err := db.DB.Raw(query, args...).Scan(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch data for export", "details": err.Error()})
		return
	}

	f := excelize.NewFile()
	sheet := "ART_Export"
	f.SetSheetName("Sheet1", sheet)

	// ART Excel format headers (Spanish)
	headers := []string{
		"Empleado",             // Employee name
		"DNI",                  // National ID
		"CUIL",                 // Tax ID
		"Fecha Nac.",           // Birth date
		"Fecha Ingreso",        // Hire date
		"Domicilio HO/Oficina", // Home/Office address
		"Dias",                 // Days (we'll use check-in date)
		"Hrs Semanales",        // Weekly hours
		"Aclaraciones",         // Notes
		"Fecha Check-in",       // Check-in date
		"Hora Check-in",        // Check-in time
		"Tipo Ubicación",       // Location type
		"Detalle Ubicación",    // Location detail
		"Tarde",                // Late
		"Motivo Tarde",         // Late reason
		"Hora Check-out",       // Check-out time
		"Estado Check-out",     // Check-out status
		"Horas Extra",          // Overtime
	}

	// Set headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	// Fill data rows
	for row, data := range rows {
		rowNum := row + 2

		// Format location as string
		locationStr := ""
		if data.Location != "" {
			// Parse JSON location if needed
			var location models.Location
			if err := json.Unmarshal([]byte(data.Location), &location); err == nil {
				locationStr = fmt.Sprintf("%s %s, %s, %s, %s",
					location.Calle, location.Numero, location.Ciudad, location.Provincia, location.Pais)
			} else {
				locationStr = data.Location
			}
		}

		// Format dates
		birthDateStr := ""
		if data.BirthDate != nil {
			birthDateStr = data.BirthDate.Format("02/01/2006")
		}

		hireDateStr := ""
		if data.HireDate != nil {
			hireDateStr = data.HireDate.Format("02/01/2006")
		}

		checkoutTimeStr := ""
		if data.CheckoutTime != nil {
			checkoutTimeStr = data.CheckoutTime.Format("15:04")
		}

		// Set cell values
		f.SetCellValue(sheet, fmt.Sprintf("A%d", rowNum), data.Empleado)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", rowNum), data.DNI)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", rowNum), data.CUIL)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", rowNum), birthDateStr)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", rowNum), hireDateStr)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", rowNum), locationStr)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", rowNum), data.Date) // Days column uses check-in date
		f.SetCellValue(sheet, fmt.Sprintf("H%d", rowNum), data.HrsSemanales)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", rowNum), data.Aclaraciones)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", rowNum), data.Date)
		f.SetCellValue(sheet, fmt.Sprintf("K%d", rowNum), data.Time.Format("15:04"))
		f.SetCellValue(sheet, fmt.Sprintf("L%d", rowNum), data.LocationType)
		f.SetCellValue(sheet, fmt.Sprintf("M%d", rowNum), data.LocationDetail)
		f.SetCellValue(sheet, fmt.Sprintf("N%d", rowNum), data.Late)
		f.SetCellValue(sheet, fmt.Sprintf("O%d", rowNum), data.LateReason)
		f.SetCellValue(sheet, fmt.Sprintf("P%d", rowNum), checkoutTimeStr)
		f.SetCellValue(sheet, fmt.Sprintf("Q%d", rowNum), data.CheckoutStatus)
		f.SetCellValue(sheet, fmt.Sprintf("R%d", rowNum), data.Overtime)
	}

	// Auto-fit columns
	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 15)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=art_export.xlsx")
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
// @Security BearerAuth
// @Param date query string true "Date (YYYY-MM-DD)"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 50, max 200)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/attendance/summary [get]
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

	// Pagination parameters
	page := 1
	pageSize := 50
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
		if pageSize < 1 || pageSize > 200 {
			pageSize = 50
		}
	}

	type attendanceRow struct {
		UserID uint
		Name   string
		Email  string
		// Checkin fields
		CheckinID        *uint
		CheckinTime      *time.Time
		Late             *bool
		LocationType     *int
		LocationDetail   *string
		Notes            *string
		LateReason       *string
		CheckinCreatedAt *time.Time
		// Absence fields
		AbsenceID        *uint
		AbsenceType      *int
		AbsenceReason    *string
		FileURL          *string
		AbsenceCreatedAt *time.Time
		// New fields
		CheckoutTime   *time.Time
		CheckoutStatus *string
		Overtime       *bool
	}

	// Get total count
	var total int64
	db.DB.Raw(`
		SELECT COUNT(*)
		FROM users u
		WHERE u.active = true AND u.pending_approval = false
	`, date, date).Scan(&total)

	var rows []attendanceRow
	db.DB.Raw(`
	SELECT
	u.id AS user_id,
	u.name,
	u.email,
	c.id AS checkin_id,
	c.time AS checkin_time,
	c.late,
	cl.location_type,
	cl.location_detail,
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
	LEFT JOIN checkins c ON c.user_id = u.id AND DATE(c.time) = ? AND c.deleted = false
	LEFT JOIN checkin_locations cl ON c.id = cl.checkin_id
	LEFT JOIN absences a ON a.user_id = u.id AND a.date = ? AND a.deleted = false
	WHERE u.active = true AND u.pending_approval = false
	ORDER BY u.name
	LIMIT ? OFFSET ?
`, date, date, pageSize, (page-1)*pageSize).Scan(&rows)

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
			// Create locations array from the query result
			var locations []models.CheckinLocation
			if r.LocationType != nil {
				locations = append(locations, models.CheckinLocation{
					LocationType:   *r.LocationType,
					LocationDetail: derefString(r.LocationDetail),
				})
			}

			checkin = &models.CheckinResponse{
				ID:             *r.CheckinID,
				UserID:         r.UserID,
				Time:           r.CheckinTime.Format("2006-01-02 15:04:05"),
				Notes:          derefString(r.Notes),
				Late:           derefBool(r.Late),
				LateReason:     derefString(r.LateReason),
				CreatedAt:      r.CheckinCreatedAt.Format("2006-01-02 15:04:05"),
				CheckoutTime:   checkoutTime,
				CheckoutStatus: derefString(r.CheckoutStatus),
				Overtime:       derefBool(r.Overtime),
				Locations:      locations,
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
				Type:      models.AbsenceType(derefInt(r.AbsenceType)),
				Reason:    derefString(r.AbsenceReason),
				FileURL:   derefString(r.FileURL),
				CreatedAt: r.AbsenceCreatedAt.Format("2006-01-02 15:04:05"),
			}
			// Optionally, override status if sick
			if r.AbsenceType != nil && *r.AbsenceType == int(models.AbsenceSick) {
				status = "sick"
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
			// Get first location type if available
			if len(checkin.Locations) > 0 {
				tableRow["location_type"] = checkin.Locations[0].LocationType
			}
			// Get first location detail if available
			if len(checkin.Locations) > 0 {
				tableRow["location_detail"] = checkin.Locations[0].LocationDetail
			}
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

	// Return paginated response with metadata
	c.JSON(200, gin.H{
		"data": tableRows,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
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

func derefInt(i *int) int {
	if i != nil {
		return *i
	}
	return 0
}

// @Summary Get daily summary
// @Description Returns the daily_summary row for a given date. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Param date query string true "Date (YYYY-MM-DD)"
// @Success 200 {object} models.DailySummary
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/dashboard/attendance/daily-summary [get]
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
// @Security BearerAuth
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
	// Initialize as empty slice to ensure we always return an array, not null
	rows := make([]map[string]interface{}, 0)
	err := db.DB.Raw("SELECT * FROM daily_checkins_view WHERE date = ?", date).Scan(&rows).Error
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to fetch checkins view", Details: err.Error()})
		return
	}

	// Always return an array, even if empty
	c.JSON(200, rows)
}

// @Summary Get individual attendance (checkin + absence + user)
// @Description Returns checkin, absence, and user info for a given user/date. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
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
	// Fix: Use DATE(time) instead of date column for checkins
	checkinErr := db.DB.Preload("Locations").Where("user_id = ? AND DATE(time) = ?", userID, date).First(&checkin).Error
	absenceErr := db.DB.Where("user_id = ? AND date = ?", userID, date).First(&absence).Error

	// Separate checkin and checkout info
	var checkinInfo interface{}
	var checkoutInfo interface{}

	if checkinErr == nil {
		// Checkin info
		checkinInfo = gin.H{
			"id":          checkin.ID,
			"user_id":     checkin.UserID,
			"time":        checkin.Time,
			"notes":       checkin.Notes,
			"late":        checkin.Late,
			"late_reason": checkin.LateReason,
			"created_at":  checkin.CreatedAt,
			"locations":   checkin.Locations,
		}

		// Checkout info (if exists)
		if checkin.CheckoutTime != nil {
			checkoutInfo = gin.H{
				"checkout_time":   checkin.CheckoutTime,
				"checkout_status": checkin.CheckoutStatus,
				"overtime":        checkin.Overtime,
			}
		}
	}

	c.JSON(200, gin.H{
		"user":     user,
		"checkin":  ifNoRecordReturnNull(checkinErr, checkinInfo),
		"checkout": checkoutInfo,
		"absence":  ifNoRecordReturnNull(absenceErr, absence),
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
// @Security BearerAuth
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
	// Try to find even soft-deleted by extracting date from time field
	// Extract date from the provided time or use today
	var checkDate string
	if req.Time != "" {
		// Try to parse the time and extract date
		if t, err := time.Parse(time.RFC3339, req.Time); err == nil {
			checkDate = t.Format("2006-01-02")
		} else {
			// Try other formats
			layouts := []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, req.Time); err == nil {
					checkDate = t.Format("2006-01-02")
					break
				}
			}
		}
	}
	if checkDate == "" {
		checkDate = time.Now().Format("2006-01-02")
	}
	err := db.DB.Unscoped().Where("user_id = ? AND DATE(time) = ?", req.UserID, checkDate).First(&checkin).Error
	if err == nil {
		// Restore if deleted
		if checkin.Deleted {
			checkin.Deleted = false
		}
		// Update fields
		checkin.Time = parseTimeOrNow(req.Time, "")
		checkin.Notes = req.Notes
		checkin.Late = req.LateReason != ""
		checkin.LateReason = req.LateReason

		// Update locations - first delete existing ones
		db.DB.Where("checkin_id = ?", checkin.ID).Delete(&models.CheckinLocation{})

		// Create new locations
		for _, loc := range req.Locations {
			location := models.CheckinLocation{
				CheckinID:      checkin.ID,
				LocationType:   loc.LocationType,
				LocationDetail: loc.LocationDetail,
			}
			db.DB.Create(&location)
		}
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
			UserID:     req.UserID,
			Time:       parseTimeOrNow(req.Time, ""),
			Notes:      req.Notes,
			Late:       req.LateReason != "",
			LateReason: req.LateReason,
		}
		if err := db.DB.Create(&checkin).Error; err != nil {
			c.JSON(500, models.ErrorResponse{Error: "Failed to create check-in", Details: err.Error()})
			return
		}

		// Create locations for the new check-in
		for _, loc := range req.Locations {
			location := models.CheckinLocation{
				CheckinID:      checkin.ID,
				LocationType:   loc.LocationType,
				LocationDetail: loc.LocationDetail,
			}
			db.DB.Create(&location)
		}
	}
	// Load locations for the response
	db.DB.Model(&checkin).Association("Locations").Find(&checkin.Locations)

	resp := models.CheckinResponse{
		ID:         checkin.ID,
		UserID:     checkin.UserID,
		Time:       checkin.Time.Format(time.RFC3339),
		Notes:      checkin.Notes,
		Late:       checkin.Late,
		LateReason: checkin.LateReason,
		CreatedAt:  checkin.CreatedAt.Format(time.RFC3339),
		Locations:  checkin.Locations,
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
	// If no time provided and no date, use current time
	if dateStr == "" {
		return time.Now()
	}
	t, _ := time.Parse("2006-01-02 15:04:05", dateStr+" 09:00:00")
	return t
}

// @Summary Get monthly analytics
// @Description Returns monthly attendance analytics. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 12, max 60)"
// @Success 200 {object} map[string]interface{}
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

	// Pagination parameters
	page := 1
	pageSize := 12
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
		if pageSize < 1 || pageSize > 60 {
			pageSize = 12
		}
	}

	// Get total count
	var total int64
	db.DB.Raw(`
		SELECT COUNT(DISTINCT to_char(time::date, 'YYYY-MM'))
		FROM checkins
		WHERE deleted = false
	`).Scan(&total)

	var stats []models.MonthlyStat
	db.DB.Raw(`
		SELECT
			to_char(time::date, 'YYYY-MM') AS month,
			COUNT(*) AS total,
			SUM(CASE WHEN late THEN 1 ELSE 0 END) AS late,
			SUM(CASE WHEN overtime THEN 1 ELSE 0 END) AS overtime
		FROM checkins
		WHERE deleted = false
		GROUP BY month
		ORDER BY month DESC
		LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize).Scan(&stats)

	c.JSON(http.StatusOK, gin.H{
		"data": stats,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// @Summary Get absence heatmap
// @Description Returns a list of dates with counts of absences. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 30, max 365)"
// @Success 200 {object} map[string]interface{}
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

	// Pagination parameters
	page := 1
	pageSize := 30
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
		if pageSize < 1 || pageSize > 365 {
			pageSize = 30
		}
	}

	// Get total count
	var total int64
	db.DB.Raw(`
		SELECT COUNT(DISTINCT date)
		FROM absences
		WHERE deleted = false
	`).Scan(&total)

	var heatmap []models.AbsenceHeatmapEntry
	db.DB.Raw(`
		SELECT date, COUNT(*) AS count
		FROM absences
		WHERE deleted = false
		GROUP BY date
		ORDER BY date DESC
		LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize).Scan(&heatmap)

	c.JSON(http.StatusOK, gin.H{
		"data": heatmap,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// @Summary Get overtime stats
// @Description Returns a list of dates with counts of overtime. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 30, max 365)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/stats/overtime [get]
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

	// Pagination parameters
	page := 1
	pageSize := 30
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
		if pageSize < 1 || pageSize > 365 {
			pageSize = 30
		}
	}

	// Get total count
	var total int64
	db.DB.Raw(`
		SELECT COUNT(DISTINCT DATE(time))
		FROM checkins
		WHERE deleted = false
	`).Scan(&total)

	var overtime []models.OvertimeStatEntry
	db.DB.Raw(`
		SELECT DATE(time) as date, SUM(CASE WHEN overtime THEN 1 ELSE 0 END) AS overtime
		FROM checkins
		WHERE deleted = false
		GROUP BY DATE(time)
		ORDER BY DATE(time) DESC
		LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize).Scan(&overtime)

	c.JSON(http.StatusOK, gin.H{
		"data": overtime,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// @Summary Get late check-in prediction
// @Description Returns a list of late check-in predictions for the next 7 days. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/analytics/prediction/late-checkins [get]
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
		SELECT DATE(time) as date, SUM(CASE WHEN late THEN 1 ELSE 0 END) AS late
		FROM checkins
		WHERE deleted = false
		GROUP BY DATE(time)
		ORDER BY DATE(time) DESC
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

// @Summary Get users by team
// @Description Returns users grouped by team. HR/admin only.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/users/by-team [get]
func getUsersByTeam(c *gin.Context) {
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

	var teamUsers []struct {
		Team  string        `json:"team"`
		Users []models.User `json:"users"`
	}

	// Get total count of teams
	var totalTeams int64
	db.DB.Model(&models.User{}).
		Distinct("team").
		Where("team IS NOT NULL AND team != ''").
		Count(&totalTeams)

	// Get teams with pagination
	var teams []string
	db.DB.Model(&models.User{}).
		Distinct("team").
		Where("team IS NOT NULL AND team != ''").
		Order("team").
		Limit(pageSize).
		Offset((page-1)*pageSize).
		Pluck("team", &teams)

	// Get users for each team (with pagination per team)
	for _, team := range teams {
		var users []models.User
		db.DB.Where("team = ? AND active = ?", team, true).
			Order("name").
			Find(&users)
		teamUsers = append(teamUsers, struct {
			Team  string        `json:"team"`
			Users []models.User `json:"users"`
		}{
			Team:  team,
			Users: users,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": teamUsers,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       totalTeams,
			"total_pages": int((totalTeams + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// @Summary Export attendance roll call to Excel (ART format)
// @Description HR/admin only. Export attendance roll call data as Excel file in ART format with all HR fields. Filters: date
// @Tags dashboard
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param date query string true "Date (YYYY-MM-DD)"
// @Success 200 {file} file
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/export/attendance [get]
func exportAttendanceToExcel(c *gin.Context) {
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
		c.JSON(400, gin.H{"error": "Missing date parameter"})
		return
	}

	// Build query to get attendance data with HR fields
	query := `
		SELECT
			u.id AS user_id,
			u.name || ' ' || u.surname as empleado,
			u.dni,
			u.cuil,
			u.birth_date,
			u.hire_date,
			u.location,
			u.weekly_hours as hrs_semanales,
			u.notes as aclaraciones,
			c.id AS checkin_id,
			c.time AS checkin_time,
			c.late,
			cl.location_type,
			cl.location_detail,
			c.notes,
			c.late_reason,
			c.checkout_time,
			c.checkout_status,
			c.overtime,
			a.id AS absence_id,
			a.type AS absence_type,
			a.reason AS absence_reason
		FROM users u
		LEFT JOIN checkins c ON c.user_id = u.id AND DATE(c.time) = ? AND c.deleted = false
		LEFT JOIN checkin_locations cl ON c.id = cl.checkin_id
		LEFT JOIN absences a ON a.user_id = u.id AND a.date = ? AND a.deleted = false
		WHERE u.active = true AND u.pending_approval = false
		ORDER BY u.name
	`

	type attendanceRow struct {
		UserID         uint       `json:"user_id"`
		Empleado       string     `json:"empleado"`
		DNI            string     `json:"dni"`
		CUIL           string     `json:"cuil"`
		BirthDate      *time.Time `json:"birth_date"`
		HireDate       *time.Time `json:"hire_date"`
		Location       string     `json:"location"`
		HrsSemanales   int        `json:"hrs_semanales"`
		Aclaraciones   string     `json:"aclaraciones"`
		CheckinID      *uint      `json:"checkin_id"`
		CheckinTime    *time.Time `json:"checkin_time"`
		Late           *bool      `json:"late"`
		LocationType   *int       `json:"location_type"`
		LocationDetail *string    `json:"location_detail"`
		Notes          *string    `json:"notes"`
		LateReason     *string    `json:"late_reason"`
		CheckoutTime   *time.Time `json:"checkout_time"`
		CheckoutStatus *string    `json:"checkout_status"`
		Overtime       *bool      `json:"overtime"`
		AbsenceID      *uint      `json:"absence_id"`
		AbsenceType    *int       `json:"absence_type"`
		AbsenceReason  *string    `json:"absence_reason"`
	}

	var rows []attendanceRow
	if err := db.DB.Raw(query, date, date).Scan(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch attendance data", "details": err.Error()})
		return
	}

	// For now, return JSON instead of Excel (Excel library not imported)
	c.JSON(http.StatusOK, gin.H{
		"message": "Excel export not implemented yet",
		"date":    date,
		"rows":    len(rows),
		"data":    rows,
	})
}

// @Summary Convert absence to checkin (HR/admin)
// @Description HR/admin can convert an absence record to a checkin record. Useful when someone was marked absent but actually arrived late. JWT with hr/admin role required.
// @Tags dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.ConvertAbsenceRequest true "Conversion request"
// @Success 200 {object} models.CheckinResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/dashboard/convert-absence-to-checkin [post]
func convertAbsenceToCheckin(c *gin.Context) {
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

	var req models.ConvertAbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}

	// Find the absence record
	var absence models.Absence
	if err := db.DB.First(&absence, req.AbsenceID).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Absence not found"})
		return
	}

	// Check if absence is already linked to a checkin
	if absence.Locked {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Cannot convert locked absence"})
		return
	}

	// Start transaction
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create checkin record
	checkinTime := parseTimeOrNow(req.CheckinTime, absence.Date)
	checkin := models.Checkin{
		UserID:     absence.UserID,
		Time:       checkinTime,
		Notes:      req.Notes,
		Late:       req.Late,
		LateReason: req.LateReason,
		AbsenceID:  &absence.ID, // Link to the absence being converted
	}

	if err := tx.Create(&checkin).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create checkin", Details: err.Error()})
		return
	}

	// Create locations if provided
	for _, loc := range req.Locations {
		location := models.CheckinLocation{
			CheckinID:      checkin.ID,
			LocationType:   loc.LocationType,
			LocationDetail: loc.LocationDetail,
		}
		if err := tx.Create(&location).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create location", Details: err.Error()})
			return
		}
	}

	// Soft delete the absence (mark as deleted)
	absence.Deleted = true
	if err := tx.Save(&absence).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update absence", Details: err.Error()})
		return
	}

	// Create audit log
	audit := models.AuditLog{
		UserEmail:  userClaims["email"].(string),
		Action:     "convert_absence_to_checkin",
		EntityID:   checkin.ID,
		EntityType: "checkin",
		OldValue:   fmt.Sprintf("absence_id:%d,type:%d,reason:%s", absence.ID, absence.Type, absence.Reason),
		NewValue:   fmt.Sprintf("checkin_id:%d,late:%t,reason:%s", checkin.ID, checkin.Late, checkin.LateReason),
		Timestamp:  time.Now(),
	}
	if err := tx.Create(&audit).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create audit log", Details: err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to commit transaction", Details: err.Error()})
		return
	}

	// Load locations for response
	if err := db.DB.Where("checkin_id = ?", checkin.ID).Find(&checkin.Locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to load locations", Details: err.Error()})
		return
	}

	resp := models.CheckinResponse{
		ID:         checkin.ID,
		UserID:     checkin.UserID,
		Time:       checkin.Time.Format(time.RFC3339),
		Notes:      checkin.Notes,
		Late:       checkin.Late,
		LateReason: checkin.LateReason,
		CreatedAt:  checkin.CreatedAt.Format(time.RFC3339),
		AbsenceID:  checkin.AbsenceID,
		Locations:  checkin.Locations,
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Create absence for user (HR/admin)
// @Description HR/admin can create an absence record for a user who didn't show up. Useful for end-of-day processing or when HR contacts user and they confirm they won't be coming. JWT with hr/admin role required.
// @Tags dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.AbsenceRequest true "Absence data"
// @Success 200 {object} models.AbsenceResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/dashboard/create-absence [post]
func createAbsenceForHR(c *gin.Context) {
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

	var req models.AbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}

	// Check if user exists and is active
	var user models.User
	if err := db.DB.Where("id = ? AND active = ?", req.UserID, true).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found or inactive"})
		return
	}

	// Check if absence already exists for this user/date
	var existingAbsence models.Absence
	if err := db.DB.Where("user_id = ? AND date = ? AND deleted = ?", req.UserID, req.Date, false).First(&existingAbsence).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "Absence already exists for this user/date"})
		return
	}

	// Check if checkin exists for this user/date
	var existingCheckin models.Checkin
	if err := db.DB.Where("user_id = ? AND DATE(time) = ? AND deleted = ?", req.UserID, req.Date, false).First(&existingCheckin).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "Checkin already exists for this user/date"})
		return
	}

	// Create absence record
	absence := models.Absence{
		UserID: req.UserID,
		Date:   req.Date,
		Type:   req.Type,
		Reason: req.Reason,
	}

	if err := db.DB.Create(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create absence", Details: err.Error()})
		return
	}

	// Create audit log
	audit := models.AuditLog{
		UserEmail:  userClaims["email"].(string),
		Action:     "create_absence",
		EntityID:   absence.ID,
		EntityType: "absence",
		OldValue:   "",
		NewValue:   fmt.Sprintf("user_id:%d,date:%s,type:%d,reason:%s", absence.UserID, absence.Date, absence.Type, absence.Reason),
		Timestamp:  time.Now(),
	}
	if err := db.DB.Create(&audit).Error; err != nil {
		// Log error but don't fail the request
		logger.Log.Error("Failed to create audit log for absence creation",
			zap.Error(err),
		)
	}

	resp := models.AbsenceResponse{
		ID:        absence.ID,
		UserID:    absence.UserID,
		Date:      absence.Date,
		Type:      absence.Type,
		Reason:    absence.Reason,
		FileURL:   absence.FileURL,
		CreatedAt: absence.CreatedAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, resp)
}
