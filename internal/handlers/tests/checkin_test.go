package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/handlers"
	"BE-ABSTI-CLOCKIN/internal/models"
)

func setupCheckinTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/checkins")
	handlers.RegisterCheckinRoutes(group)
	return engine
}

func setupCheckinTestDB() {
	dbConn, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.DB = dbConn
	db.DB.AutoMigrate(&models.User{}, &models.Checkin{})
}

func hrClaims() map[string]interface{} {
	return map[string]interface{}{"role": "hr", "user_id": 1, "email": "hr@x.com"}
}

func addHRAuth(c *gin.Context) {
	c.Set("user", hrClaims())
}

func TestCheckinAdminCRUD(t *testing.T) {
	setupCheckinTestDB()
	r := setupCheckinTestRouter()
	r.Use(func(c *gin.Context) { addHRAuth(c); c.Next() })

	// Insert a user and a check-in
	user := models.User{Email: "emp@x.com", Name: "Emp"}
	db.DB.Create(&user)
	checkin := models.Checkin{UserID: user.ID, Time: time.Now()}
	db.DB.Create(&checkin)

	// --- List all check-ins ---
	req := httptest.NewRequest("GET", "/api/checkins/all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var checkins []models.CheckinResponse
	json.Unmarshal(w.Body.Bytes(), &checkins)
	if len(checkins) != 1 {
		t.Fatalf("expected 1 checkin, got %d", len(checkins))
	}

	// --- Soft delete check-in ---
	url := fmt.Sprintf("/api/checkins/%d", checkin.ID)
	req = httptest.NewRequest("DELETE", url, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Should not appear in list
	req = httptest.NewRequest("GET", "/api/checkins/all", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &checkins)
	if len(checkins) != 0 {
		t.Fatalf("expected 0 checkins after delete, got %d", len(checkins))
	}
}
