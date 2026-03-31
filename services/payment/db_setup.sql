DROP TABLE IF EXISTS receipts;

CREATE TYPE payment_status AS ENUM ('success', 'failed');

CREATE TABLE IF NOT EXISTS receipts (
    id serial PRIMARY KEY,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    user_id INT NOT NULL,
    ammount DOUBLE PRECISION NOT NULL CHECK (ammount >= 0),
    payment_description VARCHAR(255),
    current_payment_status payment_status NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);