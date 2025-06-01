-- Events table to store raw blockchain events
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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
    ethereum_address TEXT NOT NULL,
    neutron_address TEXT,
    position_start_height BIGINT NOT NULL,
    position_end_height BIGINT,
    amount_shares TEXT NOT NULL,
    is_terminated BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create withdraw_requests table
CREATE TABLE IF NOT EXISTS withdraw_requests (
    id UUID DEFAULT gen_random_uuid(),
    withdraw_id BIGINT NOT NULL,
    contract_address TEXT NOT NULL,
    ethereum_address TEXT NOT NULL,
    amount TEXT NOT NULL,
    neutron_address TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()

);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_positions_contract_address ON positions(contract_address);
CREATE INDEX IF NOT EXISTS idx_positions_ethereum_address ON positions(ethereum_address);
CREATE INDEX IF NOT EXISTS idx_events_contract_address ON events(contract_address);

-- Enable Row Level Security
ALTER TABLE events ENABLE ROW LEVEL SECURITY;
ALTER TABLE positions ENABLE ROW LEVEL SECURITY;

-- Create policies for anonymous read access
CREATE POLICY "Allow anonymous read access on events" 
    ON events FOR SELECT 
    TO anon 
    USING (true);

CREATE POLICY "Allow anonymous read access on positions" 
    ON positions FOR SELECT 
    TO anon 
    USING (true);
