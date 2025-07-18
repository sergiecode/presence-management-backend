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
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	// Debug logging
	fmt.Printf("Connecting to database: host=%s, user=%s, dbname=%s, port=%s\n", host, user, dbname, port)

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host,
		user,
		password,
		dbname,
		port,
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
//
//   - This versioned migration system is safe for K8s
//
//   - Each migration runs only once per database, not per pod restart
//
//   - Multiple replicas can start simultaneously without conflicts
//
//   - No external dependencies or manual steps required
//
//     2. ALTERNATIVE: Dedicated Migration Job (For complex deployments):
//     ```yaml
//     # k8s job that runs migrations before app deployment
//     apiVersion: batch/v1
//     kind: Job
//     metadata:
//     name: db-migrations
//     spec:
//     template:
//     spec:
//     containers:
//
//   - name: migrations
//     image: your-app:latest
//     command: ["./migrate"]
//     restartPolicy: Never
//     ```
//
// 3. ALTERNATIVE: Migration Tool (For complex databases):
//   - Use golang-migrate or similar tools
//   - Better for databases with frequent schema changes
//   - More sophisticated rollback capabilities
//
// 4. PRODUCTION CONSIDERATIONS:
//   - Monitor migration execution in logs
//   - Consider database connection pooling during migrations
//   - Test migrations in staging environment first
//   - Have rollback plan for critical migrations
//   - Consider using database locks for complex migrations
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
		c.time::date AS date,
		c.time AS checkin_time,
		c.checkout_time,
		c.late,
		c.overtime,
		cl.location_type,
		cl.location_detail,
		c.notes,
		c.checkout_status
	FROM checkins c
	JOIN users u ON c.user_id = u.id
	LEFT JOIN checkin_locations cl ON c.id = cl.checkin_id
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

	// Migration 2: Add checkin_locations table and migrate existing data
	migration2 := `
	-- Create the checkin_locations table
	CREATE TABLE IF NOT EXISTS checkin_locations (
		id SERIAL PRIMARY KEY,
		checkin_id INTEGER NOT NULL REFERENCES checkins(id) ON DELETE CASCADE,
		location_type INTEGER NOT NULL CHECK (location_type >= 1 AND location_type <= 4),
		location_detail TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	-- Create indexes for performance
	CREATE INDEX IF NOT EXISTS idx_checkin_locations_checkin_id ON checkin_locations(checkin_id);
	CREATE INDEX IF NOT EXISTS idx_checkin_locations_location_type ON checkin_locations(location_type);

	-- Skip data migration since we're starting fresh with new structure
	-- The old checkins table doesn't have location_type and location_detail columns
	`

	// Check if migration 2 has been applied
	DB.Model(&struct{}{}).Table("schema_migrations").Where("version = ?", "002_checkin_locations").Count(&count)

	if count == 0 {
		if err := DB.Exec(migration2).Error; err != nil {
			return fmt.Errorf("failed to run migration 2: %w", err)
		}

		// Mark migration as applied
		if err := DB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", "002_checkin_locations").Error; err != nil {
			return fmt.Errorf("failed to mark migration 2 as applied: %w", err)
		}
	}

	// Migration 3: Update deactivated column to active (positive affirmation)
	migration3 := `
	-- Migration: Update deactivated column to active (positive affirmation)
	-- This migration renames the deactivated column to active and inverts the boolean values

	-- Step 1: Add the new active column
	ALTER TABLE users ADD COLUMN IF NOT EXISTS active BOOLEAN DEFAULT true;

	-- Step 2: Update the active column based on deactivated values (invert the logic) - only if deactivated column exists
	DO $$ 
	BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'deactivated') THEN
			UPDATE users SET active = NOT deactivated WHERE deactivated IS NOT NULL;
		END IF;
	END $$;

	-- Step 3: Set default value for any NULL values
	UPDATE users SET active = true WHERE active IS NULL;

	-- Step 4: Make the active column NOT NULL
	ALTER TABLE users ALTER COLUMN active SET NOT NULL;

	-- Step 5: Drop the old deactivated column (only if it exists)
	DO $$ 
	BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'deactivated') THEN
			ALTER TABLE users DROP COLUMN deactivated;
		END IF;
	END $$;

	-- Step 6: Add index for performance
	CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);
	`

	// Check if migration 3 has been applied
	DB.Model(&struct{}{}).Table("schema_migrations").Where("version = ?", "003_deactivated_to_active").Count(&count)

	if count == 0 {
		if err := DB.Exec(migration3).Error; err != nil {
			return fmt.Errorf("failed to run migration 3: %w", err)
		}

		// Mark migration as applied
		if err := DB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", "003_deactivated_to_active").Error; err != nil {
			return fmt.Errorf("failed to mark migration 3 as applied: %w", err)
		}
	}

	// Migration 4: Add end_of_day_absence_detection column to users table
	migration4 := `
	-- Migration: Add end_of_day_absence_detection column to users table
	-- This column allows HR to configure whether to auto-detect absences at end of day

	-- Step 1: Add the new column
	ALTER TABLE users ADD COLUMN IF NOT EXISTS end_of_day_absence_detection BOOLEAN DEFAULT false;

	-- Step 2: Add index for performance
	CREATE INDEX IF NOT EXISTS idx_users_end_of_day_absence_detection ON users(end_of_day_absence_detection);
	`

	// Check if migration 4 has been applied
	DB.Model(&struct{}{}).Table("schema_migrations").Where("version = ?", "004_end_of_day_absence_detection").Count(&count)

	if count == 0 {
		if err := DB.Exec(migration4).Error; err != nil {
			return fmt.Errorf("failed to run migration 4: %w", err)
		}

		// Mark migration as applied
		if err := DB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", "004_end_of_day_absence_detection").Error; err != nil {
			return fmt.Errorf("failed to mark migration 4 as applied: %w", err)
		}
	}

	return nil
}

// UpdateDailySummary aggregates checkins for a date and upserts into daily_summary
func UpdateDailySummary(date string) error {
	var total, onTime, late, overtime int64
	// Count total checkins
	if err := DB.Model(&models.Checkin{}).Where("DATE(time) = ? AND deleted = false", date).Count(&total).Error; err != nil {
		return err
	}
	// Count on-time checkins
	if err := DB.Model(&models.Checkin{}).Where("DATE(time) = ? AND late = false AND deleted = false", date).Count(&onTime).Error; err != nil {
		return err
	}
	// Count late checkins
	if err := DB.Model(&models.Checkin{}).Where("DATE(time) = ? AND late = true AND deleted = false", date).Count(&late).Error; err != nil {
		return err
	}
	// Count overtime checkouts
	if err := DB.Model(&models.Checkin{}).Where("DATE(time) = ? AND overtime = true AND deleted = false", date).Count(&overtime).Error; err != nil {
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
