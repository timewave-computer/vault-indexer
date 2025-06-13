CREATE TABLE rate_updates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rate TEXT NOT NULL,
    contract_address TEXT NOT NULL,
    block_number BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(contract_address, block_number)
);

-- Indexes for better query performance
CREATE INDEX idx_rate_updates_contract_address ON rate_updates(contract_address);

ALTER TABLE rate_updates ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Allow anonymous read access on rate_updates" 
    ON rate_updates FOR SELECT 
    TO anon 
    USING (true);
