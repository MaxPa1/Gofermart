-- +goose Up
CREATE TABLE balances (
    user_id   BIGINT         NOT NULL,
    current   NUMERIC(12, 2) NOT NULL DEFAULT 0,
    withdrawn NUMERIC(12, 2) NOT NULL DEFAULT 0,
    CONSTRAINT balances_pkey PRIMARY KEY (user_id),
    CONSTRAINT balances_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT balances_current_check CHECK (current >= 0),
    CONSTRAINT balances_withdrawn_check CHECK (withdrawn >= 0)
);

CREATE TABLE withdrawals (
    id           BIGINT         GENERATED ALWAYS AS IDENTITY,
    user_id      BIGINT         NOT NULL,
    order_number TEXT           NOT NULL,
    sum          NUMERIC(12, 2) NOT NULL,
    processed_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CONSTRAINT withdrawals_pkey PRIMARY KEY (id),
    CONSTRAINT withdrawals_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT withdrawals_order_number_check CHECK (order_number ~ '^[0-9]+$'),
    CONSTRAINT withdrawals_sum_check CHECK (sum > 0)
);

CREATE INDEX withdrawals_user_id_processed_at_idx ON withdrawals (user_id, processed_at DESC);

-- +goose Down
DROP TABLE withdrawals;
DROP TABLE balances;
