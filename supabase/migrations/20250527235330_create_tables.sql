-- Events table to store raw blockchain events
CREATE TABLE IF NOT EXISTS events (
    id SERIAL PRIMARY KEY,
    contract_address TEXT NOT NULL,
    event_name TEXT NOT NULL,
    block_number BIGINT NOT NULL,
    transaction_hash TEXT NOT NULL,
    log_index INTEGER NOT NULL,
    raw_data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(transaction_hash, log_index)
);

-- Create positions table
CREATE TABLE IF NOT EXISTS positions (
    id BIGSERIAL PRIMARY KEY,
    contract_address TEXT NOT NULL,
    event_name TEXT NOT NULL,
    block_number BIGINT NOT NULL,
    transaction_hash TEXT NOT NULL,
    -- Add other position-specific fields here
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Add indexes for common queries
    CONSTRAINT unique_position UNIQUE (contract_address, transaction_hash, block_number)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_positions_contract_address ON positions(contract_address);
CREATE INDEX IF NOT EXISTS idx_events_contract_address ON events(contract_address);
