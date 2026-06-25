CREATE TABLE IF NOT EXISTS users (
    id         BIGINT PRIMARY KEY,
    balance    NUMERIC(20, 2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transactions (
    id              BIGSERIAL PRIMARY KEY,
    transaction_id  TEXT        NOT NULL,
    user_id         BIGINT      NOT NULL REFERENCES users (id),
    source_type     TEXT        NOT NULL
        CHECK (source_type IN ('game', 'server', 'payment')),
    state           TEXT        NOT NULL CHECK (state IN ('win', 'lose')),
    amount          NUMERIC(20, 2) NOT NULL CHECK (amount > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (transaction_id)
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions (user_id);

INSERT INTO users (id, balance) VALUES
    (1, 100.00),
    (2, 250.50),
    (3, 0.00)
ON CONFLICT (id) DO NOTHING;
