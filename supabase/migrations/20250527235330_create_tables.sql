-- EVENTS
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

CREATE INDEX IF NOT EXISTS idx_events_contract_address ON events(contract_address);
ALTER TABLE events ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Allow anonymous read access on events" 
    ON events FOR SELECT 
    TO anon 
    USING (true);

-- POSITIONS
CREATE TABLE IF NOT EXISTS positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_index_id BIGINT NOT NULL,
    contract_address TEXT NOT NULL,
    owner_address TEXT NOT NULL,
    withdraw_reciever_address TEXT,
    position_start_height BIGINT NOT NULL,
    position_end_height BIGINT,
    amount_shares TEXT NOT NULL,
    is_terminated BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_positions_contract_address ON positions(contract_address);
CREATE INDEX IF NOT EXISTS idx_positions_owner_address ON positions(owner_address);

-- Enforce per-vault uniqueness of the running index
CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_contract_address_position_index_id
  ON positions(contract_address, position_index_id);
ALTER TABLE positions ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Allow anonymous read access on positions" 
    ON positions FOR SELECT 
    TO anon 
    USING (true);

-- WITHDRAW REQUESTS
CREATE TABLE IF NOT EXISTS withdraw_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    withdraw_id BIGINT NOT NULL,
    contract_address TEXT NOT NULL,
    block_number BIGINT NOT NULL,
    owner_address TEXT NOT NULL,
    amount TEXT NOT NULL,
    reciever_address TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_withdraw_requests_contract_address ON withdraw_requests(contract_address);
CREATE INDEX IF NOT EXISTS idx_withdraw_requests_owner_address ON withdraw_requests(owner_address);
CREATE INDEX IF NOT EXISTS idx_withdraw_requests_withdraw_id ON withdraw_requests(withdraw_id);



CREATE POLICY "Allow anonymous read access on withdraw_requests" 
    ON withdraw_requests FOR SELECT 
    TO anon 
    USING (true);

ALTER TABLE withdraw_requests ENABLE ROW LEVEL SECURITY;
