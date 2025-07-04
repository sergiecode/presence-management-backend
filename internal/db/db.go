package db

import (
	"fmt"
	"os"
	"time"

	"BE-ABSTI-CLOCKIN/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	return err
}

// UpdateDailySummary aggregates checkins for a date and upserts into daily_summary
func UpdateDailySummary(date string) error {
	var total, onTime, late, overtime int64
	// Count total checkins
	if err := DB.Model(&models.Checkin{}).Where("date = ? AND deleted = false", date).Count(&total).Error; err != nil {
		return err
	}
	// Count on-time checkins
	if err := DB.Model(&models.Checkin{}).Where("date = ? AND late = false AND deleted = false", date).Count(&onTime).Error; err != nil {
		return err
	}
	// Count late checkins
	if err := DB.Model(&models.Checkin{}).Where("date = ? AND late = true AND deleted = false", date).Count(&late).Error; err != nil {
		return err
	}
	// Count overtime checkouts
	if err := DB.Model(&models.Checkin{}).Where("date = ? AND overtime = true AND deleted = false", date).Count(&overtime).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	// Upsert into daily_summary
	q := `INSERT INTO daily_summary (date, total_checkins, total_on_time, total_late, total_overtime, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (date) DO UPDATE SET
			total_checkins = EXCLUDED.total_checkins,
			total_on_time = EXCLUDED.total_on_time,
			total_late = EXCLUDED.total_late,
			total_overtime = EXCLUDED.total_overtime,
			updated_at = EXCLUDED.updated_at`
	return DB.Exec(q, date, total, onTime, late, overtime, now, now).Error
}
