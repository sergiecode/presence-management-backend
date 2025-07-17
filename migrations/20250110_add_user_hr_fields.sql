-- Migration: Add HR fields to users table
-- Date: 2025-01-10
-- Description: Add fields from Excel files for HR management

-- Add new columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS dni VARCHAR(20);
ALTER TABLE users ADD COLUMN IF NOT EXISTS cuil VARCHAR(20);
ALTER TABLE users ADD COLUMN IF NOT EXISTS birth_date DATE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS hire_date DATE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS location JSONB;
ALTER TABLE users ADD COLUMN IF NOT EXISTS weekly_hours INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS notes TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS team VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS zoho_access BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS teams_access BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS on_site_required BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS weekly_objective_days INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS monthly_objective_days INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS office_days VARCHAR(50);

-- Add indexes for better performance
CREATE INDEX IF NOT EXISTS idx_users_dni ON users(dni);
CREATE INDEX IF NOT EXISTS idx_users_cuil ON users(cuil);
CREATE INDEX IF NOT EXISTS idx_users_team ON users(team);
CREATE INDEX IF NOT EXISTS idx_users_hire_date ON users(hire_date); 