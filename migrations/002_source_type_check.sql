ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS transactions_source_type_check;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_source_type_check
        CHECK (source_type IN ('game', 'server', 'payment'));
