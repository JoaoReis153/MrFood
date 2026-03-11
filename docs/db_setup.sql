CREATE TABLE location(
    location_id serial PRIMARY KEY,
    latitude FLOAT NOT NULL,
    longitude FLOAT NOT NULL,
    address VARCHAR(100),
);

CREATE TABLE restaurant(
    restaurant_id serial PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    tags VARCHAR(50),
    price VARCHAR(50),
    phone VARCHAR(15),
    avg_rating INT,
    sponsored BOOLEAN,
    location INT references location(location_id)
);

CREATE TABLE review(
    review_id serial PRIMARY KEY,
    rating INT NOT NULL,
    comment VARCHAR(50),
    categories VARCHAR()
);

