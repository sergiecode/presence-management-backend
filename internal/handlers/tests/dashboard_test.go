package handlers_test

import (
	"encoding/json"
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

func setupDashboardTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/dashboard")
	handlers.RegisterDashboardRoutes(group)
	return engine
}

func setupDashboardTestDB() {
	dbConn, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.DB = dbConn
	db.DB.AutoMigrate(&models.User{}, &models.Checkin{}, &models.Absence{}, &models.AuditLog{})
}

func TestDashboardEndpoints(t *testing.T) {
	setupDashboardTestDB()
	r := setupDashboardTestRouter()

	// Seed data
	user := models.User{Email: "hr@absti.com", Name: "HR", Role: "hr"}
	db.DB.Create(&user)
	checkin := models.Checkin{UserID: user.ID, Time: time.Now()}
	db.DB.Create(&checkin)
	absence := models.Absence{UserID: user.ID, Date: time.Now().Format("2006-01-02"), Type: "absence", Reason: "Vacation"}
	db.DB.Create(&absence)
	db.DB.Create(&models.AuditLog{UserEmail: user.Email, Action: "test", EntityID: 1, EntityType: "user", OldValue: "{}", NewValue: "{}", Timestamp: time.Now()})

	t.Run("attendance stats HR", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/dashboard/attendance", nil)
		addHRAuth(c)
		r.HandleContext(c)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		if resp["total_checkins"] == nil {
			t.Fatalf("missing stats")
		}
	})

	t.Run("attendance stats forbidden for employee", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/dashboard/attendance", nil)
		c.Set("user", map[string]interface{}{"user_id": 2.0, "role": "employee"})
		r.HandleContext(c)
		if w.Code != 403 {
			t.Fatalf("expected 403, got %d", w.Code)
		}
	})

	t.Run("absence stats HR", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/dashboard/absences", nil)
		addHRAuth(c)
		r.HandleContext(c)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("user stats HR", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/dashboard/users", nil)
		addHRAuth(c)
		r.HandleContext(c)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("audit logs HR", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/dashboard/audit-logs", nil)
		addHRAuth(c)
		r.HandleContext(c)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		if resp["logs"] == nil {
			t.Fatalf("missing logs")
		}
	})
}
