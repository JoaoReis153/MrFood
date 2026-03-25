DROP TABLE IF EXISTS sponsor;

CREATE TABLE sponsorship(
    restaurant_id serial PRIMARY KEY,
    tier INT,
    sections VARCHAR(50)[],
    status BOOLEAN,
    until DATE
);