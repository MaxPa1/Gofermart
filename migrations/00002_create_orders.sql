-- +goose Up
CREATE TABLE orders (
    number      TEXT           NOT NULL,
    user_id     BIGINT         NOT NULL,
    status      TEXT           NOT NULL DEFAULT 'NEW',
    accrual     NUMERIC(12, 2) NOT NULL DEFAULT 0,
    uploaded_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CONSTRAINT orders_pkey PRIMARY KEY (number),
    CONSTRAINT orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT orders_number_check CHECK (number ~ '^[0-9]+$'),
    CONSTRAINT orders_status_check CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),
    CONSTRAINT orders_accrual_check CHECK (accrual >= 0)
);

CREATE INDEX orders_user_id_uploaded_at_idx ON orders (user_id, uploaded_at DESC);

-- +goose Down
DROP TABLE orders;
