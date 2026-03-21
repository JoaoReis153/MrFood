DROP TABLE IF EXISTS review;

CREATE TABLE review(
    review_id serial PRIMARY KEY,
    restaurant_id INT NOT NULL,
    user_id INT NOT NULL,
    comment VARCHAR(100) NOT NULL,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW() NOT NULL,
);

CREATE TABLE user(
    user_id serial PRIMARY KEY,
    name VARCHAR(50) NOT NULL
);