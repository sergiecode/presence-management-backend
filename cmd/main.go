// @title ABSTI Presence API
// @version 1.0
// @description ABSTI Presence API for employee attendance management
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT token (Swagger UI will automatically add "Bearer" prefix)

package main

import (
	"BE-ABSTI-CLOCKIN/internal/handlers"
	"fmt"
	"log"
	"os"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"BE-ABSTI-CLOCKIN/pkg/jwtutil"

	_ "BE-ABSTI-CLOCKIN/docs"

	"flag"

	"BE-ABSTI-CLOCKIN/internal/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/crypto/bcrypt"
)

func ensureAdminUser() {
	// First, try to find any existing admin user
	var admin models.User
	err := db.DB.Where("email = ?", "admin@absti.com").First(&admin).Error
	
	if err == nil {
		// Admin exists, ensure it's properly configured
		needsUpdate := false
		if admin.PendingApproval {
			admin.PendingApproval = false
			needsUpdate = true
		}
		if admin.Deactivated {
			admin.Deactivated = false
			needsUpdate = true
		}
		if !admin.EmailConfirmed {
			admin.EmailConfirmed = true
			needsUpdate = true
		}
		if admin.Role != "admin" {
			admin.Role = "admin"
			needsUpdate = true
		}
		
		if needsUpdate {
			if err := db.DB.Save(&admin).Error; err != nil {
				log.Printf("Failed to update admin user: %v", err)
			} else {
				log.Println("Admin user updated and properly configured")
			}
		} else {
			log.Println("Admin user already exists and properly configured")
		}
		return
	}
	
	// Admin doesn't exist, create it
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
	
	if err := db.DB.Create(&admin).Error; err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}
	log.Println("Admin user created successfully")
}

func main() {
	if err := logger.Init(); err != nil {
		log.Fatalf("failed to init zap logger: %v", err)
	}
	defer logger.Log.Sync()

	// CLI flag for summary aggregation
	var summaryDate string
	flag.StringVar(&summaryDate, "summary-date", "", "Aggregate daily summary for this date (YYYY-MM-DD) and exit")
	flag.Parse()

	if summaryDate != "" {
		if err := db.Connect(); err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		if err := db.UpdateDailySummary(summaryDate); err != nil {
			log.Fatalf("failed to update daily summary for %s: %v", summaryDate, err)
		}
		fmt.Printf("Daily summary for %s updated successfully\n", summaryDate)
		return
	}

	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error loading .env")
	}

	if err := db.Connect(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Run GORM AutoMigrate first to create tables
	if err := db.DB.AutoMigrate(&models.User{}, &models.Checkin{}, &models.Absence{}, &models.AuditLog{}, &models.RefreshToken{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	
	// Run SQL migrations after tables are created
	if err := db.RunSQLMigrations(); err != nil {
		log.Fatalf("failed to run SQL migrations: %v", err)
	}

	// Add compound index for (user_id, date) if not exists
	db.DB.Exec("CREATE INDEX IF NOT EXISTS idx_checkins_user_date ON checkins (user_id, date);")

	ensureAdminUser()

	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	fmt.Println(string(hash))

	r := gin.Default()

	// CORS middleware - must be before routes
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:8081", "http://localhost:3001"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	handlers.RegisterAuthRoutes(r)

	// All /api endpoints require JWT
	auth := r.Group("/api")
	auth.Use(jwtutil.JWTAuthMiddleware())
	handlers.RegisterUserRoutes(auth.Group("/users"))
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
