package main

import (
	"BE-ABSTI-CLOCKIN/internal/handlers"
	"fmt"
	"log"
	"os"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"BE-ABSTI-CLOCKIN/pkg/jwtutil"

	_ "BE-ABSTI-CLOCKIN/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/crypto/bcrypt"
)

func ensureAdminUser() {
	var admin models.User
	err := db.DB.Where("role = ? AND deactivated = ?", "admin", false).First(&admin).Error
	if err == nil {
		// Ensure admin is not pending approval or deactivated
		if admin.PendingApproval || admin.Deactivated {
			admin.PendingApproval = false
			admin.Deactivated = false
			db.DB.Save(&admin)
			log.Println("Admin user re-activated and approved")
		}
		log.Println("Admin user already exists")
		return
	}
	// If not found, create admin
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash admin password: %v", err)
	}
	admin = models.User{
		Email:           "admin@absti.com",
		Name:            "Admin",
		Role:            "admin",
		PasswordHash:    string(hash),
		EmailConfirmed:  true,
		PendingApproval: false,
		Deactivated:     false,
	}
	if err := db.DB.Where(models.User{Email: admin.Email}).FirstOrCreate(&admin).Error; err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}
	log.Println("Admin user created or already present")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error loading .env")
	}

	if err := db.Connect(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.DB.AutoMigrate(&models.User{}, &models.Checkin{}, &models.Absence{}, &models.AuditLog{}, &models.RefreshToken{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	ensureAdminUser()

	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	fmt.Println(string(hash))

	r := gin.Default()

	r.Use(cors.Default())

	handlers.RegisterAuthRoutes(r)

	// Open /api/users for dev bootstrap
	handlers.RegisterUserRoutes(r.Group("/api/users"))

	// All other /api endpoints require JWT
	auth := r.Group("/api")
	auth.Use(jwtutil.JWTAuthMiddleware())
	handlers.RegisterCheckinRoutes(auth.Group("/checkins"))
	handlers.RegisterAbsenceRoutes(auth.Group("/absences"))
	handlers.RegisterDashboardRoutes(auth.Group("/dashboard"))
	auth.GET("/protected", func(c *gin.Context) {
		claims, _ := c.Get("user")
		c.JSON(200, gin.H{"message": "You are authenticated!", "claims": claims})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	r.Run(":" + port)
}
