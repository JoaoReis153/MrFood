CREATE TABLE IF NOT EXISTS sponsorship(
    restaurant_id INT PRIMARY KEY,
    tier INT,
    until DATE
);

CREATE TABLE IF NOT EXISTS restaurant_categories (
    id SERIAL PRIMARY KEY,
    restaurant_id INT NOT NULL REFERENCES sponsorship(restaurant_id) ON DELETE CASCADE,
    category TEXT NOT NULL
);