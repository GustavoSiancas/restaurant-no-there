ALTER TABLE foods ALTER COLUMN total_calories TYPE DOUBLE PRECISION;
ALTER TABLE foods ADD CONSTRAINT foods_total_calories_finite
    CHECK (total_calories < 'Infinity'::DOUBLE PRECISION);
