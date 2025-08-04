DROP INDEX IF EXISTS idx_positions_contract_address;
DROP INDEX IF EXISTS idx_positions_owner_address;

DROP INDEX IF EXISTS idx_withdraw_requests_contract_address;
DROP INDEX IF EXISTS idx_withdraw_requests_owner_address;
DROP INDEX IF EXISTS idx_withdraw_requests_withdraw_id;

-- create more efficient indexes

CREATE INDEX IF NOT EXISTS idx_withdraw_requests_withdraw_id ON withdraw_requests(contract_address, withdraw_id);
CREATE INDEX IF NOT EXISTS idx_withdraw_requests_receiver_address ON withdraw_requests(contract_address, receiver_address);
CREATE INDEX IF NOT EXISTS idx_withdraw_requests_owner_address ON withdraw_requests(contract_address, owner_address);


CREATE INDEX IF NOT EXISTS idx_positions_owner_address ON positions(contract_address, owner_address);


CREATE INDEX IF NOT EXISTS idx_vault_rates ON rate_updates(contract_address);
