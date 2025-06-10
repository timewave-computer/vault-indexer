-- Add constraint to ensure position_end_height is required when is_terminated is not null
ALTER TABLE positions
    ADD CONSTRAINT check_position_end_height_when_terminated
    CHECK (
        (is_terminated IS NOT NULL AND position_end_height IS NOT NULL) OR
        (is_terminated IS NULL)
    );

-- Add constraint to ensure position_end_height is required when withdraw_receiver_address is not null
ALTER TABLE positions
    ADD CONSTRAINT check_position_end_height_when_withdraw_receiver
    CHECK (
        (withdraw_receiver_address IS NOT NULL AND position_end_height IS NOT NULL) OR
        (withdraw_receiver_address IS NULL)
    ); 