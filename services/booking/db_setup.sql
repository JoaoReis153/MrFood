DROP TABLE IF EXISTS booking;
DROP TABLE IF EXISTS restaurant_slots;

CREATE TABLE IF NOT EXISTS booking(
    id serial PRIMARY KEY,
    user_id BIGINT NOT NULL,
    restaurant_id BIGINT NOT NULL,
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    people_count INT NOT NULL
);

CREATE TABLE IF NOT EXISTS restaurant_slots(
    id SERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL,
    time_start TIMESTAMP NOT NULL,
    time_end TIMESTAMP NOT NULL,
    max_slots INT NOT NULL,
    current_slots INT DEFAULT 0,
    CHECK (current_slots <= max_slots),
    UNIQUE (restaurant_id, time_start)
);

CREATE INDEX IF NOT EXISTS idx_already_booked ON booking (user_id, restaurant_id, time_start);
CREATE INDEX IF NOT EXISTS idx_restaurant_slots ON restaurant_slots (restaurant_id, time_start);

CREATE OR REPLACE FUNCTION handle_booking_insert()
RETURNS TRIGGER AS $$
BEGIN
    -- Try to reserve capacity only when this booking still fits.
    UPDATE restaurant_slots
    SET current_slots = current_slots + NEW.people_count
    WHERE restaurant_id = NEW.restaurant_id
      AND time_start = NEW.time_start
      AND current_slots + NEW.people_count <= max_slots;

    IF FOUND THEN
        RETURN NEW;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM restaurant_slots
        WHERE restaurant_id = NEW.restaurant_id
          AND time_start = NEW.time_start
    ) THEN
        RAISE WARNING 'Skipping booking id %, restaurant_id %, time_start %, requested seats %: not enough available slots',
            NEW.id,
            NEW.restaurant_id,
            NEW.time_start,
            NEW.people_count;
        RETURN NULL;
    END IF;

    IF NEW.people_count > 50 THEN
        RAISE WARNING 'Skipping booking id %, restaurant_id %, time_start %, requested seats %: exceeds max slot size %',
            NEW.id,
            NEW.restaurant_id,
            NEW.time_start,
            NEW.people_count,
            50;
        RETURN NULL;
    END IF;

    INSERT INTO restaurant_slots (restaurant_id, time_start, time_end, max_slots, current_slots)
    VALUES (NEW.restaurant_id, NEW.time_start, NEW.time_end, 50, NEW.people_count);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_booking_insert
BEFORE INSERT ON booking
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