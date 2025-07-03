CREATE TABLE IF NOT EXISTS block_finality (
    block_tag TEXT PRIMARY KEY,
    last_confirmed_block_number BIGINT NOT NULL DEFAULT 0,
);

ALTER TABLE block_finality ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Allow anonymous read access on block_finality" 
    ON block_finality FOR SELECT 
    TO anon 
    USING (true);