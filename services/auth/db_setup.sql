CREATE TABLE app_user(
     user_id serial PRIMARY KEY,
     username VARCHAR(50) NOT NULL UNIQUE,
     password VARCHAR(60) NOT NULL,
     email VARCHAR(50) NOT NULL UNIQUE
);