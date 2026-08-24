-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL,
    username TEXT,
    group_id INT
);

-- +goose Down
DROP TABLE IF EXISTS users;
