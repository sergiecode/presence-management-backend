-- Migration to update enum fields from strings to integers
-- This migration converts the existing string-based enum values to integer IDs

-- Update absence types
UPDATE absences SET type = 1 WHERE type = 'Licencia por maternidad';
UPDATE absences SET type = 2 WHERE type = 'Licencia por enfermedad';
UPDATE absences SET type = 3 WHERE type = 'Ausente por enfermedad';
UPDATE absences SET type = 4 WHERE type = 'Ausente por enfermedad familiar';
UPDATE absences SET type = 5 WHERE type = 'Ausente por día de estudio/examen';
UPDATE absences SET type = 6 WHERE type = 'Ausente por duelo';
UPDATE absences SET type = 7 WHERE type = 'Día por mudanza';
UPDATE absences SET type = 8 WHERE type = 'Vacaciones';
UPDATE absences SET type = 9 WHERE type = 'late';
UPDATE absences SET type = 10 WHERE type = 'medical';
UPDATE absences SET type = 11 WHERE type = 'absence';

-- Update location types in users table (location JSONB field)
-- Note: This is more complex as it's in a JSONB field, so we'll handle it in the application layer
-- The Location struct will handle the conversion automatically

-- Alter column types to integer
ALTER TABLE absences ALTER COLUMN type TYPE INTEGER USING type::integer;

-- Add comments for documentation
COMMENT ON COLUMN absences.type IS 'Absence type enum: 1=Maternity, 2=Sick Leave, 3=Sick Absence, 4=Family Sick, 5=Study, 6=Bereavement, 7=Moving, 8=Vacation, 9=Late, 10=Medical, 11=General'; 