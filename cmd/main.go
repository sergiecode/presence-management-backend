package main

import (
	"BE-ABSTI-CLOCKIN/internal/handlers"
	"log"
	"os"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"

	"BE-ABSTI-CLOCKIN/pkg/jwtutil"

	_ "BE-ABSTI-CLOCKIN/docs"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error loading .env")
	}

	if err := db.Connect(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.DB.AutoMigrate(&models.User{}, &models.Checkin{}, &models.Absence{}, &models.AuditLog{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	r := gin.Default()

	handlers.RegisterAuthRoutes(r)
	handlers.RegisterHealthRoute(r)

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
