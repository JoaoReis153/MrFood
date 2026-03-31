DROP TABLE IF EXISTS booking;
DROP TABLE IF EXISTS restaurant_slots;

CREATE TABLE IF NOT EXISTS booking(
    id serial PRIMARY KEY,
    user_id INT NOT NULL,
    restaurant_id INT NOT NULL,
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    people_count INT NOT NULL,
    CHECK (time_start < time_end)
);

CREATE TABLE IF NOT EXISTS restaurant_slots(
    id SERIAL PRIMARY KEY,
    restaurant_id INT NOT NULL,
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    max_slots INT NOT NULL,
    current_slots INT DEFAULT 0,
    CHECK (time_start < time_end),
    CHECK (current_slots <= max_slots),
    UNIQUE (restaurant_id, time_start)
);

CREATE INDEX IF NOT EXISTS idx_already_booked ON booking (user_id, restaurant_id, time_start);
CREATE INDEX IF NOT EXISTS idx_restaurant_slots ON restaurant_slots (restaurant_id, time_start);

CREATE OR REPLACE FUNCTION handle_booking_insert()
RETURNS TRIGGER AS $$
DECLARE
    v_max_slots INT;
BEGIN
    v_max_slots := current_setting('app.max_slots')::int;

    INSERT INTO restaurant_slots (
        restaurant_id,
        time_start,
        time_end,
        max_slots,
        current_slots
    )
    VALUES (
        NEW.restaurant_id,
        NEW.time_start,
        NEW.time_end,
        v_max_slots,
        NEW.people_count
    )
    ON CONFLICT (restaurant_id, time_start)
    DO UPDATE
    SET current_slots = restaurant_slots.current_slots + NEW.people_count
    WHERE restaurant_slots.current_slots + NEW.people_count <= restaurant_slots.max_slots;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'not enough slots';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_booking_insert
AFTER INSERT ON booking
FOR EACH ROW
EXECUTE FUNCTION handle_booking_insert();


CREATE OR REPLACE FUNCTION handle_booking_delete()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE restaurant_slots
    SET current_slots = current_slots - OLD.people_count
    WHERE restaurant_id = OLD.restaurant_id
      AND time_start = OLD.time_start;

    DELETE FROM restaurant_slots
    WHERE current_slots <= 0;

    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_booking_delete
AFTER DELETE ON booking
FOR EACH ROW
EXECUTE FUNCTION handle_booking_delete();