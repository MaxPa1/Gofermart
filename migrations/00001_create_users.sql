-- +goose Up
CREATE TABLE users (
    id            BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_login_key UNIQUE (login)
);

-- +goose Down
DROP TABLE users;