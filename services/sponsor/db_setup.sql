CREATE TABLE IF NOT EXISTS sponsorship(
    restaurant_id NUMERIC(22, 0) PRIMARY KEY,
    tier INT,
    until DATE
);

CREATE TABLE IF NOT EXISTS restaurant_categories (
    id SERIAL PRIMARY KEY,
    restaurant_id NUMERIC(22, 0) NOT NULL REFERENCES sponsorship(restaurant_id) ON DELETE CASCADE,
    category TEXT NOT NULL
);