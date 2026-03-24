CREATE TABLE booking(
    id serial PRIMARY KEY,
    user_id INT NOT NULL,
    restaurant_id INT NOT NULL,
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    people_count INT NOT NULL,
    CHECK (time_start < time_end),
);

CREATE TABLE restaurant_slots(
    id SERIAL PRIMARY KEY,
    restaurant_id INT NOT NULL,
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    max_slots INT NOT NULL,
    current_slots INT DEFAULT 0,
    CHECK (time_start < time_end),
    CHECK (current_slots <= max_slots),
);

CREATE INDEX idx_already_booked ON booking (restaurant_id, time_start);
CREATE INDEX idx_restaurant_slots ON restaurant_slots (restaurant_id, time_start);