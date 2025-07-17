-- Migration: Add checkin_locations table and remove date column from checkins
-- This enables multiple work locations per day for employees

-- Step 1: Create the checkin_locations table
CREATE TABLE IF NOT EXISTS checkin_locations (
    id SERIAL PRIMARY KEY,
    checkin_id INTEGER NOT NULL REFERENCES checkins(id) ON DELETE CASCADE,
    location_type INTEGER NOT NULL CHECK (location_type >= 1 AND location_type <= 4),
    location_detail TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Step 2: Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_checkin_locations_checkin_id ON checkin_locations(checkin_id);
CREATE INDEX IF NOT EXISTS idx_checkin_locations_location_type ON checkin_locations(location_type);

-- Step 3: Migrate existing data from checkins to checkin_locations
-- This preserves existing location data before removing the date column
INSERT INTO checkin_locations (checkin_id, location_type, location_detail, created_at)
SELECT 
    id as checkin_id,
    location_type,
    location_detail,
    created_at
FROM checkins 
WHERE location_type IS NOT NULL 
  AND location_type > 0;

-- Step 4: Remove the date column from checkins (it's redundant with time)
-- Note: This will fail if there are any views or functions depending on the date column
-- We'll handle that in the next step

-- Step 5: Update the daily_checkins_view to work with the new structure
DROP VIEW IF EXISTS daily_checkins_view;

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

-- Step 6: Update the daily_summary table to work with the new structure
-- (This will be handled by the existing migration system)

-- Migration completed successfully
-- Note: The date column removal will be done in a separate step to avoid breaking existing code 