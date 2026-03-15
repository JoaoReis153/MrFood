DROP TABLE IF EXISTS location CASCADE;
DROP TABLE IF EXISTS restaurant CASCADE;
DROP TABLE IF EXISTS app_user CASCADE;
DROP TABLE IF EXISTS review;
DROP TABLE IF EXISTS reservation;

CREATE TABLE location(
    location_id serial PRIMARY KEY,
    latitude FLOAT NOT NULL,
    longitude FLOAT NOT NULL,
    address VARCHAR(100)
);

CREATE TABLE restaurant(
    restaurant_id serial PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    password VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(50) NOT NULL UNIQUE,
    tags VARCHAR(50),
    price VARCHAR(50),
    phone VARCHAR(15),
    avg_rating INT,
    sponsored BOOLEAN,
    location_id INT references location(location_id)
);

CREATE TABLE app_user(
    user_id serial PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE review(
    review_id serial PRIMARY KEY,
    rating INT NOT NULL,
    comment VARCHAR(50),
    categories VARCHAR(50),
    timestamp TIMESTAMP,
    user_id INT references app_user(user_id),
    restaurant_id INT references restaurant(restaurant_id)
);

CREATE TABLE reservation(
    reservation_id serial PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    people_count INT NOT NULL,
    user_id INT references app_user(user_id),
    restaurant_id INT references restaurant(restaurant_id)
);
