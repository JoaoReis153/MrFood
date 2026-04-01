DROP TABLE IF EXISTS review;
DROP TABLE IF EXISTS restaurant_stats;

CREATE TABLE IF NOT EXISTS review(
    review_id     SERIAL PRIMARY KEY,
    restaurant_id INT NOT NULL,
    user_id       INT NOT NULL,
    comment       VARCHAR(100) NOT NULL,
    rating        INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    created_at    TIMESTAMP DEFAULT NOW() NOT NULL,
    CONSTRAINT unique_user_restaurant UNIQUE (restaurant_id, user_id)
);

CREATE TABLE IF NOT EXISTS restaurant_stats(
    restaurant_id  INT PRIMARY KEY,
    average_rating DECIMAL(3,2) NOT NULL,
    review_count INT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_review_restaurant_rating ON review(restaurant_id, rating);

CREATE OR REPLACE FUNCTION update_restaurant_stats() RETURNS TRIGGER AS $$
DECLARE
    target_id INT;
    v_avg DECIMAL(3,2);
    v_count INT;
BEGIN
    target_id := COALESCE(NEW.restaurant_id, OLD.restaurant_id);
    IF (TG_OP = 'INSERT' OR TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND NEW.rating <> OLD.rating)) THEN
        SELECT ROUND(AVG(rating)::numeric, 2), COUNT(*)
        INTO v_avg, v_count
        FROM review
        WHERE restaurant_id = target_id;
        IF v_count > 0 THEN
            INSERT INTO restaurant_stats (restaurant_id, average_rating, review_count)
            VALUES (target_id, v_avg, v_count)
            ON CONFLICT (restaurant_id) DO UPDATE
            SET average_rating = EXCLUDED.average_rating,
                review_count = EXCLUDED.review_count;
        ELSE
            DELETE FROM restaurant_stats WHERE restaurant_id = target_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_restaurant_stats ON review;
CREATE TRIGGER trg_update_restaurant_stats
AFTER INSERT OR UPDATE OR DELETE ON review
FOR EACH ROW EXECUTE FUNCTION update_restaurant_stats();