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
    c.gps_lat,
    c.gps_long,
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