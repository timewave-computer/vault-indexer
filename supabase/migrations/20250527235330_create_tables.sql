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
    position_index_number BIGINT NOT NULL,
    contract_address TEXT NOT NULL,
    ethereum_address TEXT NOT NULL,
    neutron_address TEXT,
    position_start_height BIGINT NOT NULL,
    position_end_height BIGINT,
    amount NUMERIC NOT NULL,
    is_terminated BOOLEAN NOT NULL DEFAULT FALSE,
    entry_method TEXT NOT NULL CHECK (entry_method IN ('deposit', 'transfer')),
    exit_method TEXT CHECK (exit_method IN ('withdraw', 'deposit', 'transfer')),
    transaction_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Add indexes for common queries
    CONSTRAINT unique_position UNIQUE (contract_address, transaction_hash, position_start_height)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_positions_contract_address ON positions(contract_address);
CREATE INDEX IF NOT EXISTS idx_positions_ethereum_address ON positions(ethereum_address);
CREATE INDEX IF NOT EXISTS idx_events_contract_address ON events(contract_address);
