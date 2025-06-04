-- Remove existing unique constraint
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_block_number_log_index_key;

-- Add new columns
ALTER TABLE events 
    ADD COLUMN IF NOT EXISTS last_updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS is_processed BOOLEAN;

-- Add new unique constraint
ALTER TABLE events ADD CONSTRAINT events_block_number_log_index_key 
    UNIQUE(block_number, log_index);
