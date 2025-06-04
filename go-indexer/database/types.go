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
	OwnerAddress            string  `json:"owner_address"`
	Id                      string  `json:"id"`
	IsTerminated            *bool   `json:"is_terminated"`
	WithdrawRecieverAddress *string `json:"withdraw_reciever_address"`
	PositionEndHeight       *int64  `json:"position_end_height"`
	PositionIndexId         int64   `json:"position_index_id"`
	PositionStartHeight     int64   `json:"position_start_height"`
}

type PublicPositionsInsert struct {
	AmountShares            string  `json:"amount_shares"`
	ContractAddress         string  `json:"contract_address"`
	OwnerAddress            string  `json:"owner_address"`
	IsTerminated            *bool   `json:"is_terminated"`
	WithdrawRecieverAddress *string `json:"withdraw_reciever_address"`
	PositionEndHeight       *int64  `json:"position_end_height"`
	PositionIndexId         int64   `json:"position_index_id"`
	PositionStartHeight     int64   `json:"position_start_height"`
}

type PublicPositionsUpdate struct {
	Id                      string  `json:"id"`
	IsTerminated            *bool   `json:"is_terminated"`
	WithdrawRecieverAddress *string `json:"withdraw_reciever_address"`
	PositionEndHeight       *int64  `json:"position_end_height"`
}

type PublicWithdrawRequestsSelect struct {
	Amount          string `json:"amount"`
	ContractAddress string `json:"contract_address"`
	CreatedAt       string `json:"created_at"`
	EthereumAddress string `json:"ethereum_address"`
	Id              string `json:"id"`
	WithdrawId      int64  `json:"withdraw_id"`
	OwnerAddress    string `json:"owner_address"`
	RecieverAddress string `json:"reciever_address"`
	BlockNumber     int64  `json:"block_number"`
}

type PublicWithdrawRequestsInsert struct {
	Amount          string `json:"amount"`
	ContractAddress string `json:"contract_address"`
	OwnerAddress    string `json:"owner_address"`
	RecieverAddress string `json:"reciever_address"`
	BlockNumber     int64  `json:"block_number"`
	WithdrawId      int64  `json:"withdraw_id"`
}

type PublicWithdrawRequestsUpdate struct {
	Amount          *string `json:"amount"`
	ContractAddress *string `json:"contract_address"`
	CreatedAt       *string `json:"created_at"`
	OwnerAddress    *string `json:"owner_address"`
	RecieverAddress *string `json:"reciever_address"`
	BlockNumber     *int64  `json:"block_number"`
	WithdrawId      *int64  `json:"withdraw_id"`
}
