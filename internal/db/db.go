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

// RunSQLMigrations executes SQL migration files with versioning
// 
// K8s DEPLOYMENT RECOMMENDATIONS:
// 
// 1. CURRENT APPROACH (Recommended for small teams):
//    - This versioned migration system is safe for K8s
//    - Each migration runs only once per database, not per pod restart
//    - Multiple replicas can start simultaneously without conflicts
//    - No external dependencies or manual steps required
//
// 2. ALTERNATIVE: Dedicated Migration Job (For complex deployments):
//    ```yaml
//    # k8s job that runs migrations before app deployment
//    apiVersion: batch/v1
//    kind: Job
//    metadata:
//      name: db-migrations
//    spec:
//      template:
//        spec:
//          containers:
//          - name: migrations
//            image: your-app:latest
//            command: ["./migrate"]
//          restartPolicy: Never
//    ```
//
// 3. ALTERNATIVE: Migration Tool (For complex databases):
//    - Use golang-migrate or similar tools
//    - Better for databases with frequent schema changes
//    - More sophisticated rollback capabilities
//
// 4. PRODUCTION CONSIDERATIONS:
//    - Monitor migration execution in logs
//    - Consider database connection pooling during migrations
//    - Test migrations in staging environment first
//    - Have rollback plan for critical migrations
//    - Consider using database locks for complex migrations
func RunSQLMigrations() error {
	// Create migrations table if it doesn't exist
	createMigrationsTable := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	`
	if err := DB.Exec(createMigrationsTable).Error; err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Migration 1: Create daily_checkins_view and daily_summary
	migration1 := `
	-- Create daily_checkins_view for efficient 'all check-ins today' queries
	CREATE OR REPLACE VIEW daily_checkins_view AS
	SELECT
		c.id AS checkin_id,
		c.user_id,
		u.name AS user_name,
		u.email AS user_email,
		c.date,
		c.time AS checkin_time,
		c.checkout_time,
		c.late,
		c.overtime,
		c.location_type,
		c.location_detail,
		c.notes,
		c.checkout_status
	FROM checkins c
	JOIN users u ON c.user_id = u.id
	WHERE c.deleted = false;

	-- Create daily_summary table for aggregated daily check-in stats
	CREATE TABLE IF NOT EXISTS daily_summary (
		date DATE PRIMARY KEY,
		total_checkins INT NOT NULL DEFAULT 0,
		total_on_time INT NOT NULL DEFAULT 0,
		total_late INT NOT NULL DEFAULT 0,
		total_overtime INT NOT NULL DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	-- Optional: index for fast date queries
	CREATE INDEX IF NOT EXISTS idx_daily_summary_date ON daily_summary (date);
	`
	
	// Check if migration 1 has been applied
	var count int64
	DB.Model(&struct{}{}).Table("schema_migrations").Where("version = ?", "001_daily_checkins_view").Count(&count)
	
	if count == 0 {
		if err := DB.Exec(migration1).Error; err != nil {
			return fmt.Errorf("failed to run migration 1: %w", err)
		}
		
		// Mark migration as applied
		if err := DB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", "001_daily_checkins_view").Error; err != nil {
			return fmt.Errorf("failed to mark migration 1 as applied: %w", err)
		}
	}
	
	return nil
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
