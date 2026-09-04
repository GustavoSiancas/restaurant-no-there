CREATE TABLE foods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    long_description TEXT NOT NULL,
    total_calories INTEGER NOT NULL,
    photo_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT foods_name_not_blank CHECK (BTRIM(name) <> ''),
    CONSTRAINT foods_long_description_not_blank CHECK (BTRIM(long_description) <> ''),
    CONSTRAINT foods_total_calories_non_negative CHECK (total_calories >= 0),
    CONSTRAINT foods_photo_url_not_blank CHECK (BTRIM(photo_url) <> '')
);

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tags_name_not_blank CHECK (BTRIM(name) <> '')
);

CREATE UNIQUE INDEX tags_name_unique_idx ON tags (LOWER(BTRIM(name)));

CREATE TABLE food_tags (
    food_id UUID NOT NULL REFERENCES foods(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (food_id, tag_id)
);

CREATE INDEX food_tags_tag_id_idx ON food_tags (tag_id);
