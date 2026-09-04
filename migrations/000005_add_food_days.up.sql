CREATE TYPE food_day_status AS ENUM ('OPEN', 'CLOSED');

CREATE TABLE food_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_date DATE NOT NULL,
    meal_type meal_type NOT NULL,
    food_id UUID NOT NULL REFERENCES foods(id) ON DELETE RESTRICT,
    status food_day_status NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- No unique constraint: any number of foods may be assigned to a meal on a date.
CREATE INDEX food_days_date_meal_type_idx ON food_days (service_date, meal_type);
CREATE INDEX food_days_food_id_idx ON food_days (food_id);
