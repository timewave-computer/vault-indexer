package database

type PublicEventsSelect struct {
	BlockNumber     int64       `json:"block_number"`
	ContractAddress string      `json:"contract_address"`
	CreatedAt       *string     `json:"created_at"`
	EventName       string      `json:"event_name"`
	Id              string      `json:"id"`
	LogIndex        int32       `json:"log_index"`
	RawData         interface{} `json:"raw_data"`
	TransactionHash string      `json:"transaction_hash"`
}

type PublicEventsInsert struct {
	BlockNumber     int64       `json:"block_number"`
	ContractAddress string      `json:"contract_address"`
	EventName       string      `json:"event_name"`
	LogIndex        int32       `json:"log_index"`
	RawData         interface{} `json:"raw_data"`
	TransactionHash string      `json:"transaction_hash"`
}

type PublicEventsUpdate struct {
	BlockNumber     *int64      `json:"block_number"`
	ContractAddress *string     `json:"contract_address"`
	CreatedAt       *string     `json:"created_at"`
	EventName       *string     `json:"event_name"`
	Id              *string     `json:"id"`
	LogIndex        *int32      `json:"log_index"`
	RawData         interface{} `json:"raw_data"`
	TransactionHash *string     `json:"transaction_hash"`
}

type PublicPositionsSelect struct {
	AmountShares            string  `json:"amount_shares"`
	ContractAddress         string  `json:"contract_address"`
	CreatedAt               string  `json:"created_at"`
	Id                      string  `json:"id"`
	IsTerminated            *bool   `json:"is_terminated"`
	OwnerAddress            string  `json:"owner_address"`
	PositionEndHeight       *int64  `json:"position_end_height"`
	PositionIndexId         int64   `json:"position_index_id"`
	PositionStartHeight     int64   `json:"position_start_height"`
	WithdrawRecieverAddress *string `json:"withdraw_reciever_address"`
}

type PublicPositionsInsert struct {
	AmountShares            string  `json:"amount_shares"`
	ContractAddress         string  `json:"contract_address"`
	IsTerminated            *bool   `json:"is_terminated"`
	OwnerAddress            string  `json:"owner_address"`
	PositionEndHeight       *int64  `json:"position_end_height"`
	PositionIndexId         int64   `json:"position_index_id"`
	PositionStartHeight     int64   `json:"position_start_height"`
	WithdrawRecieverAddress *string `json:"withdraw_reciever_address"`
}

type PublicPositionsUpdate struct {
	AmountShares            *string `json:"amount_shares"`
	ContractAddress         *string `json:"contract_address"`
	CreatedAt               *string `json:"created_at"`
	Id                      *string `json:"id"`
	IsTerminated            *bool   `json:"is_terminated"`
	OwnerAddress            *string `json:"owner_address"`
	PositionEndHeight       *int64  `json:"position_end_height"`
	PositionIndexId         *int64  `json:"position_index_id"`
	PositionStartHeight     *int64  `json:"position_start_height"`
	WithdrawRecieverAddress *string `json:"withdraw_reciever_address"`
}

type PublicWithdrawRequestsSelect struct {
	Amount          string `json:"amount"`
	BlockNumber     int64  `json:"block_number"`
	ContractAddress string `json:"contract_address"`
	CreatedAt       string `json:"created_at"`
	Id              string `json:"id"`
	OwnerAddress    string `json:"owner_address"`
	RecieverAddress string `json:"reciever_address"`
	WithdrawId      int64  `json:"withdraw_id"`
}

type PublicWithdrawRequestsInsert struct {
	Amount          string `json:"amount"`
	BlockNumber     int64  `json:"block_number"`
	ContractAddress string `json:"contract_address"`
	OwnerAddress    string `json:"owner_address"`
	RecieverAddress string `json:"reciever_address"`
	WithdrawId      int64  `json:"withdraw_id"`
}

type PublicWithdrawRequestsUpdate struct {
	Amount          *string `json:"amount"`
	BlockNumber     *int64  `json:"block_number"`
	ContractAddress *string `json:"contract_address"`
	CreatedAt       *string `json:"created_at"`
	Id              *string `json:"id"`
	OwnerAddress    *string `json:"owner_address"`
	RecieverAddress *string `json:"reciever_address"`
	WithdrawId      *int64  `json:"withdraw_id"`
}
