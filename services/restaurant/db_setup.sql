CREATE TABLE IF NOT EXISTS restaurants (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    address VARCHAR(100) NOT NULL,
    media_url VARCHAR(255),
    max_slots INTEGER NOT NULL CHECK (max_slots >= 0),
    owner_id INTEGER NOT NULL,
    sponsor_tier INTEGER NOT NULL DEFAULT 0 CHECK (sponsor_tier >= 0)
);

CREATE TABLE IF NOT EXISTS restaurant_working_hours (
    id SERIAL PRIMARY KEY,

    restaurant_id INTEGER NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    working_hour TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS restaurant_categories (
    id SERIAL PRIMARY KEY,
    restaurant_id INTEGER NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    category TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_restaurant_working_hours_restaurant_id
    ON restaurant_working_hours (restaurant_id,);

CREATE INDEX IF NOT EXISTS idx_restaurant_categories_restaurant_id
    ON restaurant_categories (restaurant_id);

