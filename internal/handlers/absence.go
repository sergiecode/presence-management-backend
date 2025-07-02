package handlers

import (
	"fmt"
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/logger"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func RegisterAbsenceRoutes(r *gin.RouterGroup) {
	r.POST("/", reportAbsence)
	r.GET("/", getAbsenceHistory)
	r.PUT("/:id", updateAbsence)
	r.POST("/:id/documents", uploadAbsenceDocument)
	r.GET("/:id/documents", getAbsenceDocuments)
	r.PATCH("/:id/lock", lockAbsence)
	r.GET("/all", listAllAbsences)
	r.DELETE("/:id", deleteAbsence)
	r.POST("/batch-approve", batchApproveAbsences)
}

// @Summary Report absence/late/medical
// @Description User reports absence, late arrival, or medical leave. JWT required.
// @Tags absence
// @Accept json
// @Produce json
// @Param absence body models.AbsenceRequest true "Absence data"
// @Success 200 {object} models.AbsenceResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/absences [post]
func reportAbsence(c *gin.Context) {
	var req models.AbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Error("Invalid absence request",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, ok := userClaims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid token"})
		return
	}
	absence := models.Absence{
		UserID: uint(userID),
		Date:   req.Date,
		Type:   req.Type,
		Reason: req.Reason,
	}
	if err := db.DB.Create(&absence).Error; err != nil {
		logger.Log.Error("Failed to create absence",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create absence", Details: err.Error()})
		return
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

// @Summary Get user's absence history
// @Description Returns all absences for the authenticated user, newest first. JWT required.
// @Tags absence
// @Produce json
// @Success 200 {array} models.AbsenceResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/absences [get]
func getAbsenceHistory(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, ok := userClaims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid token"})
		return
	}
	var absences []models.Absence
	err := db.DB.Where("user_id = ?", uint(userID)).Order("date desc").Find(&absences).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch absences", Details: err.Error()})
		return
	}
	resp := make([]models.AbsenceResponse, len(absences))
	for i, ab := range absences {
		resp[i] = models.AbsenceResponse{
			ID:        ab.ID,
			UserID:    ab.UserID,
			Date:      ab.Date,
			Type:      ab.Type,
			Reason:    ab.Reason,
			FileURL:   ab.FileURL,
			CreatedAt: ab.CreatedAt.Format(time.RFC3339),
		}
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Update absence/late/medical
// @Description User can update their own absence if not locked. HR/admin can update any. JWT required.
// @Tags absence
// @Accept json
// @Produce json
// @Param id path int true "Absence ID"
// @Param absence body models.AbsenceRequest true "Absence data"
// @Success 200 {object} models.AbsenceResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/absences/{id} [put]
func updateAbsence(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, _ := userClaims["user_id"].(float64)
	role, _ := userClaims["role"].(string)
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil {
		logger.Log.Error("Absence not found",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Absence not found"})
		return
	}
	if role != "hr" && role != "admin" && absence.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "Forbidden: can only update your own absence"})
		return
	}
	if absence.Locked && role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "Absence is locked by HR"})
		return
	}
	var req models.AbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Error("Invalid absence request",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	absence.Date = req.Date
	absence.Type = req.Type
	absence.Reason = req.Reason
	if err := db.DB.Save(&absence).Error; err != nil {
		logger.Log.Error("Failed to update absence",
			zap.String("endpoint", c.FullPath()),
			zap.String("method", c.Request.Method),
			zap.String("user", getUserEmail(c)),
			zap.Any("payload", req),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update absence", Details: err.Error()})
		return
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

// @Summary Upload medical certificate
// @Description User uploads a file for their own absence if not locked. Only PDF/JPG/PNG, max 5MB. JWT required.
// @Tags absence
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Absence ID"
// @Param file formData file true "Medical certificate file"
// @Success 200 {object} models.AbsenceResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/absences/{id}/documents [post]
func uploadAbsenceDocument(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, _ := userClaims["user_id"].(float64)
	role, _ := userClaims["role"].(string)
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Absence not found"})
		return
	}
	if role != "hr" && role != "admin" && absence.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "Forbidden: can only upload for your own absence"})
		return
	}
	if absence.Locked && role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "Absence is locked by HR"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "File required"})
		return
	}
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "File too large (max 5MB)"})
		return
	}
	ext := file.Filename[len(file.Filename)-4:]
	if ext != ".pdf" && ext != ".jpg" && ext != ".png" && ext != "jpeg" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Only PDF, JPG, PNG allowed"})
		return
	}
	path := "uploads/absence_" + id + "_" + file.Filename
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to save file", Details: err.Error()})
		return
	}
	absence.FileURL = "/" + path
	if err := db.DB.Save(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update absence with file", Details: err.Error()})
		return
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

func getAbsenceDocuments(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "get absence documents (stub)"})
}

// @Summary Lock or unlock an absence
// @Description HR/admin can lock or unlock an absence to prevent user edits. JWT with hr/admin required.
// @Tags absence
// @Accept json
// @Produce json
// @Param id path int true "Absence ID"
// @Param lock body models.LockAbsenceRequest true "Lock state"
// @Success 200 {object} models.AbsenceResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/absences/{id}/lock [patch]
func lockAbsence(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "Forbidden: HR or admin only"})
		return
	}
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Absence not found"})
		return
	}
	var req models.LockAbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	absence.Locked = req.Locked
	if err := db.DB.Save(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update lock state", Details: err.Error()})
		return
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

// @Summary List all absences (HR/admin)
// @Description HR/admin only. Returns paginated list of all absences. Query params: page, page_size, user_id, date, type
// @Tags absence
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20)"
// @Param user_id query int false "Filter by user ID"
// @Param date query string false "Filter by date (YYYY-MM-DD)"
// @Param type query string false "Filter by type (absence/late/medical)"
// @Success 200 {object} []models.AbsenceResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/absences/all [get]
func listAllAbsences(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
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
	var absences []models.Absence
	q := db.DB.Where("deleted = ?", false)
	if v := c.Query("user_id"); v != "" {
		q = q.Where("user_id = ?", v)
	}
	if v := c.Query("date"); v != "" {
		q = q.Where("date = ?", v)
	}
	if v := c.Query("type"); v != "" {
		q = q.Where("type = ?", v)
	}
	q = q.Order("date desc").Offset((page - 1) * pageSize).Limit(pageSize)
	q.Find(&absences)
	resp := make([]models.AbsenceResponse, len(absences))
	for i, ab := range absences {
		resp[i] = models.AbsenceResponse{
			ID:        ab.ID,
			UserID:    ab.UserID,
			Date:      ab.Date,
			Type:      ab.Type,
			Reason:    ab.Reason,
			FileURL:   ab.FileURL,
			CreatedAt: ab.CreatedAt.Format(time.RFC3339),
		}
	}
	c.JSON(200, resp)
}

// @Summary Soft delete absence (HR/admin)
// @Description HR/admin only. Soft delete an absence by setting deleted=true.
// @Tags absence
// @Produce json
// @Param id path int true "Absence ID"
// @Success 200 {object} gin.H
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/absences/{id} [delete]
func deleteAbsence(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(401, models.ErrorResponse{Error: "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(403, models.ErrorResponse{Error: "Forbidden: HR or admin only"})
		return
	}
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil || absence.Deleted {
		c.JSON(404, models.ErrorResponse{Error: "Absence not found"})
		return
	}
	absence.Deleted = true
	if err := db.DB.Save(&absence).Error; err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to delete absence", Details: err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Absence deleted"})
}

// POST /api/absences/batch-approve
func batchApproveAbsences(c *gin.Context) {
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
	userClaims := claims.(map[string]interface{})
	role, _ := userClaims["role"].(string)
	// userEmail, _ := userClaims["email"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(403, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}

	tx := db.DB.Begin()
	var processed, failed int
	for _, id := range req.IDs {
		var ab models.Absence
		if err := tx.First(&ab, id).Error; err != nil || ab.Deleted {
			failed++
			continue
		}
		// You can add more status fields if needed, for now just log action
		ab.Reason = req.Reason
		ab.Locked = true // lock after decision
		if err := tx.Save(&ab).Error; err != nil {
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
	c.JSON(200, gin.H{"success": true, "processed": processed, "failed": failed, "message": fmt.Sprintf("Processed %d absences", processed)})
}

func getUserEmail(c *gin.Context) string {
	claims, ok := c.Get("user")
	if !ok {
		return ""
	}
	userClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	email, _ := userClaims["email"].(string)
	return email
}
