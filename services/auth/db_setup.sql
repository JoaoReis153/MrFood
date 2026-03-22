CREATE TABLE app_user(
     user_id serial PRIMARY KEY,
     username VARCHAR(50) NOT NULL UNIQUE,
     password VARCHAR(50) NOT NULL UNIQUE,
     email VARCHAR(50) NOT NULL UNIQUE
);