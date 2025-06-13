package database

type PositionUpdate struct {
	Id                      string  `json:"id"`
	IsTerminated            bool    `json:"is_terminated"`
	PositionEndHeight       int64   `json:"position_end_height"`
	WithdrawReceiverAddress *string `json:"withdraw_receiver_address"`
}

func ToPositionUpdate(u PublicPositionsUpdate) PositionUpdate {
	// omits empty values so they are not attempted to be updated
	return PositionUpdate{
		Id:                      *u.Id,
		IsTerminated:            *u.IsTerminated,
		PositionEndHeight:       *u.PositionEndHeight,
		WithdrawReceiverAddress: u.WithdrawReceiverAddress,
	}
}

type PositionInsert struct {
	PositionIndexId         int64   `json:"position_index_id"`
	OwnerAddress            string  `json:"owner_address"`
	ContractAddress         string  `json:"contract_address"`
	AmountShares            string  `json:"amount_shares"`
	PositionStartHeight     int64   `json:"position_start_height"`
	WithdrawReceiverAddress *string `json:"withdraw_receiver_address"`
}

func ToPositionInsert(u PublicPositionsInsert) PositionInsert {
	return PositionInsert{
		PositionIndexId:         u.PositionIndexId,
		OwnerAddress:            u.OwnerAddress,
		ContractAddress:         u.ContractAddress,
		AmountShares:            u.AmountShares,
		PositionStartHeight:     u.PositionStartHeight,
		WithdrawReceiverAddress: nil,
	}
}
