CREATE TABLE booking(
    id serial PRIMARY KEY,
    user_id INT NOT NULL,
    restaurant_id INT NOT NULL
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    people_count INT NOT NULL,
);