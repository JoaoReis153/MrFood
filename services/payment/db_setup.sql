DROP TABLE IF EXISTS receipts;

CREATE TABLE IF NOT EXISTS receipts (
    id serial PRIMARY KEY,
    user_id INT NOT NULL,
    ammount DOUBLE PRECISION NOT NULL CHECK (ammount >= 0),
    payment_description VARCHAR(255)
);