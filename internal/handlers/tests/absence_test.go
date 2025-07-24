package handlers_test

// import (
// 	"encoding/json"
// 	"fmt"
// 	"net/http/httptest"
// 	"testing"

// 	"github.com/gin-gonic/gin"
// 	"gorm.io/driver/sqlite"
// 	"gorm.io/gorm"

// 	"BE-ABSTI-CLOCKIN/internal/db"
// 	"BE-ABSTI-CLOCKIN/internal/handlers"
// 	"BE-ABSTI-CLOCKIN/internal/models"
// )

// func setupAbsenceTestRouter() *gin.Engine {
// 	gin.SetMode(gin.TestMode)
// 	engine := gin.New()
// 	group := engine.Group("/api/absences")
// 	handlers.RegisterAbsenceRoutes(group)
// 	return engine
// }

// func setupAbsenceTestDB() {
// 	dbConn, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
// 	db.DB = dbConn
// 	db.DB.AutoMigrate(&models.User{}, &models.Absence{})
// }

// func TestAbsenceAdminCRUD(t *testing.T) {
// 	setupAbsenceTestDB()
// 	r := setupAbsenceTestRouter()

// 	// Seed user and absences
// 	user := models.User{Email: "emp@absti.com", Name: "Emp", Role: "employee"}
// 	db.DB.Create(&user)
// 	absence := models.Absence{UserID: user.ID, Date: "2024-06-20", Type: "absence", Reason: "Sick"}
// 	db.DB.Create(&absence)

// 	t.Run("list all absences", func(t *testing.T) {
// 		w := httptest.NewRecorder()
// 		c, _ := gin.CreateTestContext(w)
// 		c.Request = httptest.NewRequest("GET", "/api/absences/all", nil)
// 		addHRAuth(c)
// 		r.HandleContext(c)
// 		if w.Code != 200 {
// 			t.Fatalf("expected 200, got %d", w.Code)
// 		}
// 		var resp []models.AbsenceResponse
// 		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
// 			t.Fatalf("bad json: %v", err)
// 		}
// 		if len(resp) == 0 || resp[0].Reason != "Sick" {
// 			t.Fatalf("expected seeded absence in list")
// 		}
// 	})

// 	t.Run("soft delete absence", func(t *testing.T) {
// 		w := httptest.NewRecorder()
// 		c, _ := gin.CreateTestContext(w)
// 		c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(absence.ID)}}
// 		c.Request = httptest.NewRequest("DELETE", "/api/absences/1", nil)
// 		addHRAuth(c)
// 		r.HandleContext(c)
// 		if w.Code != 200 {
// 			t.Fatalf("expected 200, got %d", w.Code)
// 		}
// 		var out map[string]interface{}
// 		_ = json.Unmarshal(w.Body.Bytes(), &out)
// 		if out["message"] != "Absence deleted" {
// 			t.Fatalf("unexpected response: %v", out)
// 		}
// 		// Confirm it's deleted
// 		var ab models.Absence
// 		db.DB.First(&ab, absence.ID)
// 		if !ab.Deleted {
// 			t.Fatalf("absence not marked deleted")
// 		}
// 	})
// }
//
