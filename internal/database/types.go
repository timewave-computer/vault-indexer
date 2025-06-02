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
	AmountShares        string  `json:"amount_shares"`
	ContractAddress     string  `json:"contract_address"`
	CreatedAt           string  `json:"created_at"`
	EthereumAddress     string  `json:"ethereum_address"`
	Id                  int64   `json:"id"`
	IsTerminated        *bool   `json:"is_terminated"`
	NeutronAddress      *string `json:"neutron_address"`
	PositionEndHeight   *int64  `json:"position_end_height"`
	PositionStartHeight int64   `json:"position_start_height"`
}

type PublicPositionsInsert struct {
	AmountShares        string  `json:"amount_shares"`
	ContractAddress     string  `json:"contract_address"`
	EthereumAddress     string  `json:"ethereum_address"`
	IsTerminated        *bool   `json:"is_terminated"`
	NeutronAddress      *string `json:"neutron_address"`
	PositionEndHeight   *int64  `json:"position_end_height"`
	PositionStartHeight int64   `json:"position_start_height"`
}

type PublicPositionsUpdate struct {
	AmountShares        *string `json:"amount_shares"`
	ContractAddress     *string `json:"contract_address"`
	CreatedAt           *string `json:"created_at"`
	EthereumAddress     *string `json:"ethereum_address"`
	Id                  *int64  `json:"id"`
	IsTerminated        *bool   `json:"is_terminated"`
	NeutronAddress      *string `json:"neutron_address"`
	PositionEndHeight   *int64  `json:"position_end_height"`
	PositionStartHeight *int64  `json:"position_start_height"`
}

type PublicWithdrawRequestsSelect struct {
	Amount          string  `json:"amount"`
	ContractAddress string  `json:"contract_address"`
	EthereumAddress string  `json:"ethereum_address"`
	Id              *string `json:"id"`
	NeutronAddress  string  `json:"neutron_address"`
	WithdrawId      int64   `json:"withdraw_id"`
}

type PublicWithdrawRequestsInsert struct {
	Amount          string `json:"amount"`
	ContractAddress string `json:"contract_address"`
	EthereumAddress string `json:"ethereum_address"`
	NeutronAddress  string `json:"neutron_address"`
	WithdrawId      int64  `json:"withdraw_id"`
}

type PublicWithdrawRequestsUpdate struct {
	Amount          *string `json:"amount"`
	ContractAddress *string `json:"contract_address"`
	EthereumAddress *string `json:"ethereum_address"`
	Id              *string `json:"id"`
	NeutronAddress  *string `json:"neutron_address"`
	WithdrawId      *int64  `json:"withdraw_id"`
}
