CREATE TABLE rate_updates (
    id SERIAL PRIMARY KEY,
    rate NUMERIC NOT NULL,
    contract_address VARCHAR(42) NOT NULL,
    event_id UUID NOT NULL REFERENCES events(id),
    block_number BIGINT NOT NULL,
    transaction_hash VARCHAR(66) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(event_id)
);

-- Indexes for better query performance
CREATE INDEX idx_rate_updates_contract_address ON rate_updates(contract_address);
