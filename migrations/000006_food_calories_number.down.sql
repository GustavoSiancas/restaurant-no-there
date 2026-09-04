ALTER TABLE foods DROP CONSTRAINT foods_total_calories_finite;
ALTER TABLE foods ALTER COLUMN total_calories TYPE INTEGER USING ROUND(total_calories)::INTEGER;
