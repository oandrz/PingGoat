-- +goose Up
CREATE TABLE users
(
    id               uuid Primary Key default gen_random_uuid(),
    email            varchar(255) unique not null,
    password_hash    varchar(255)        not null,
    github_token_enc text,
    created_at       timestamptz         not null default now(),
    updated_at       timestamptz         not null default now()
);

-- +goose Down
DROP TABLE IF EXISTS users;
