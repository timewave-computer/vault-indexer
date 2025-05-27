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

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    address TEXT PRIMARY KEY,
    first_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Positions table
CREATE TABLE IF NOT EXISTS positions (
    id SERIAL PRIMARY KEY,
    account_address TEXT NOT NULL REFERENCES accounts(address),
    contract_address TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    last_updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_address, contract_address)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_events_contract_address ON events(contract_address);
CREATE INDEX IF NOT EXISTS idx_events_block_number ON events(block_number);
CREATE INDEX IF NOT EXISTS idx_positions_account_address ON positions(account_address);
CREATE INDEX IF NOT EXISTS idx_positions_contract_address ON positions(contract_address); 