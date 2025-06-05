// MOVE FILE to internal/handlers/tests/user_test.go

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/handlers"
	"BE-ABSTI-CLOCKIN/internal/models"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// Use a test group for /api/users
	group := engine.Group("/api/users")
	handlers.RegisterUserRoutes(group)
	return engine
}

func setupTestDB() {
	dbConn, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.DB = dbConn
	db.DB.AutoMigrate(&models.User{})
}

func adminClaims() map[string]interface{} {
	return map[string]interface{}{"role": "admin", "user_id": 1, "email": "admin@x.com"}
}

func addAdminAuth(c *gin.Context) {
	c.Set("user", adminClaims())
}

func TestUserCRUD(t *testing.T) {
	setupTestDB()
	r := setupTestRouter()
	// Gin middleware to inject admin claims
	r.Use(func(c *gin.Context) { addAdminAuth(c); c.Next() })

	// --- Create user ---
	user := models.User{Email: "test@x.com", Name: "Test User"}
	body, _ := json.Marshal(user)
	req := httptest.NewRequest("POST", "/api/users/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var created models.User
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, created.Email)
	}

	// --- List users ---
	req = httptest.NewRequest("GET", "/api/users/", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var users []models.User
	json.Unmarshal(w.Body.Bytes(), &users)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	// --- Get user by ID ---
	url := "/api/users/" + itoa(created.ID)
	req = httptest.NewRequest("GET", url, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got models.User
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ID != created.ID {
		t.Fatalf("expected ID %d, got %d", created.ID, got.ID)
	}

	// --- Update user ---
	update := models.User{Name: "Updated Name"}
	body, _ = json.Marshal(update)
	req = httptest.NewRequest("PUT", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Name != "Updated Name" {
		t.Fatalf("expected updated name, got %s", got.Name)
	}

	// --- Soft delete user ---
	req = httptest.NewRequest("DELETE", url, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Should not appear in list
	req = httptest.NewRequest("GET", "/api/users/", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &users)
	if len(users) != 0 {
		t.Fatalf("expected 0 users after delete, got %d", len(users))
	}
}

// Helper for int to string (since strconv.Itoa is not imported)
func itoa(i uint) string {
	return fmt.Sprintf("%d", i)
}
