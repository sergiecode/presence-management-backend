package handlers

import (
	"net/http"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"github.com/gin-gonic/gin"
)

func RegisterAbsenceRoutes(r *gin.RouterGroup) {
	r.POST("/", reportAbsence)
	r.GET("/", getAbsenceHistory)
	r.PUT("/:id", updateAbsence)
	r.POST("/:id/documents", uploadAbsenceDocument)
	r.GET("/:id/documents", getAbsenceDocuments)
	r.PATCH("/:id/lock", lockAbsence)
}

// @Summary Report absence/late/medical
// @Description User reports absence, late arrival, or medical leave. JWT required.
// @Tags absence
// @Accept json
// @Produce json
// @Param absence body models.AbsenceRequest true "Absence data"
// @Success 200 {object} models.AbsenceResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Router /api/absences [post]
func reportAbsence(c *gin.Context) {
	var req models.AbsenceRequest
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
	userID, ok := userClaims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	absence := models.Absence{
		UserID: uint(userID),
		Date:   req.Date,
		Type:   req.Type,
		Reason: req.Reason,
	}
	if err := db.DB.Create(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create absence", "details": err.Error()})
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
// @Failure 401 {object} gin.H
// @Router /api/absences [get]
func getAbsenceHistory(c *gin.Context) {
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
	var absences []models.Absence
	err := db.DB.Where("user_id = ?", uint(userID)).Order("date desc").Find(&absences).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch absences", "details": err.Error()})
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
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/absences/{id} [put]
func updateAbsence(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, _ := userClaims["user_id"].(float64)
	role, _ := userClaims["role"].(string)
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Absence not found"})
		return
	}
	if role != "hr" && role != "admin" && absence.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: can only update your own absence"})
		return
	}
	if absence.Locked && role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Absence is locked by HR"})
		return
	}
	var req models.AbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	absence.Date = req.Date
	absence.Type = req.Type
	absence.Reason = req.Reason
	if err := db.DB.Save(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update absence", "details": err.Error()})
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
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/absences/{id}/documents [post]
func uploadAbsenceDocument(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	userID, _ := userClaims["user_id"].(float64)
	role, _ := userClaims["role"].(string)
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Absence not found"})
		return
	}
	if role != "hr" && role != "admin" && absence.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: can only upload for your own absence"})
		return
	}
	if absence.Locked && role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Absence is locked by HR"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File required"})
		return
	}
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 5MB)"})
		return
	}
	ext := file.Filename[len(file.Filename)-4:]
	if ext != ".pdf" && ext != ".jpg" && ext != ".png" && ext != "jpeg" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only PDF, JPG, PNG allowed"})
		return
	}
	path := "uploads/absence_" + id + "_" + file.Filename
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file", "details": err.Error()})
		return
	}
	absence.FileURL = "/" + path
	if err := db.DB.Save(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update absence with file", "details": err.Error()})
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
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/absences/{id}/lock [patch]
func lockAbsence(c *gin.Context) {
	claims, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userClaims := claims.(map[string]interface{})
	role, _ := userClaims["role"].(string)
	if role != "hr" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: HR or admin only"})
		return
	}
	id := c.Param("id")
	var absence models.Absence
	if err := db.DB.First(&absence, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Absence not found"})
		return
	}
	var req models.LockAbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	absence.Locked = req.Locked
	if err := db.DB.Save(&absence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update lock state", "details": err.Error()})
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
