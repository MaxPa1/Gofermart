-- +goose Up
ALTER TABLE orders ADD COLUMN checked_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE orders DROP COLUMN checked_at;
