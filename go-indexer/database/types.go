package database

type PublicEventsSelect struct {
  BlockHash       string      `json:"block_hash"`
  BlockNumber     int64       `json:"block_number"`
  ContractAddress string      `json:"contract_address"`
  CreatedAt       *string     `json:"created_at"`
  EventName       string      `json:"event_name"`
  Id              string      `json:"id"`
  IsProcessed     *bool       `json:"is_processed"`
  LastUpdatedAt   *string     `json:"last_updated_at"`
  LogIndex        int32       `json:"log_index"`
  RawData         interface{} `json:"raw_data"`
  TransactionHash string      `json:"transaction_hash"`
}

type PublicEventsInsert struct {
  BlockHash       string      `json:"block_hash"`
  BlockNumber     int64       `json:"block_number"`
  ContractAddress string      `json:"contract_address"`
  CreatedAt       *string     `json:"created_at"`
  EventName       string      `json:"event_name"`
  Id              *string     `json:"id"`
  IsProcessed     *bool       `json:"is_processed"`
  LastUpdatedAt   *string     `json:"last_updated_at"`
  LogIndex        int32       `json:"log_index"`
  RawData         interface{} `json:"raw_data"`
  TransactionHash string      `json:"transaction_hash"`
}

type PublicEventsUpdate struct {
  BlockHash       *string     `json:"block_hash"`
  BlockNumber     *int64      `json:"block_number"`
  ContractAddress *string     `json:"contract_address"`
  CreatedAt       *string     `json:"created_at"`
  EventName       *string     `json:"event_name"`
  Id              *string     `json:"id"`
  IsProcessed     *bool       `json:"is_processed"`
  LastUpdatedAt   *string     `json:"last_updated_at"`
  LogIndex        *int32      `json:"log_index"`
  RawData         interface{} `json:"raw_data"`
  TransactionHash *string     `json:"transaction_hash"`
}

type PublicPositionsSelect struct {
  AmountShares            string  `json:"amount_shares"`
  ContractAddress         string  `json:"contract_address"`
  CreatedAt               string  `json:"created_at"`
  Id                      string  `json:"id"`
  IsTerminated            bool    `json:"is_terminated"`
  OwnerAddress            string  `json:"owner_address"`
  PositionEndHeight       *int64  `json:"position_end_height"`
  PositionIndexId         int64   `json:"position_index_id"`
  PositionStartHeight     int64   `json:"position_start_height"`
  WithdrawReceiverAddress *string `json:"withdraw_receiver_address"`
}

type PublicPositionsInsert struct {
  AmountShares            string  `json:"amount_shares"`
  ContractAddress         string  `json:"contract_address"`
  CreatedAt               *string `json:"created_at"`
  Id                      *string `json:"id"`
  IsTerminated            *bool   `json:"is_terminated"`
  OwnerAddress            string  `json:"owner_address"`
  PositionEndHeight       *int64  `json:"position_end_height"`
  PositionIndexId         int64   `json:"position_index_id"`
  PositionStartHeight     int64   `json:"position_start_height"`
  WithdrawReceiverAddress *string `json:"withdraw_receiver_address"`
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
  WithdrawReceiverAddress *string `json:"withdraw_receiver_address"`
}

type PublicWithdrawRequestsSelect struct {
  Amount          string `json:"amount"`
  BlockNumber     int64  `json:"block_number"`
  ContractAddress string `json:"contract_address"`
  CreatedAt       string `json:"created_at"`
  Id              string `json:"id"`
  OwnerAddress    string `json:"owner_address"`
  ReceiverAddress string `json:"receiver_address"`
  WithdrawId      int64  `json:"withdraw_id"`
}

type PublicWithdrawRequestsInsert struct {
  Amount          string  `json:"amount"`
  BlockNumber     int64   `json:"block_number"`
  ContractAddress string  `json:"contract_address"`
  CreatedAt       *string `json:"created_at"`
  Id              *string `json:"id"`
  OwnerAddress    string  `json:"owner_address"`
  ReceiverAddress string  `json:"receiver_address"`
  WithdrawId      int64   `json:"withdraw_id"`
}

type PublicWithdrawRequestsUpdate struct {
  Amount          *string `json:"amount"`
  BlockNumber     *int64  `json:"block_number"`
  ContractAddress *string `json:"contract_address"`
  CreatedAt       *string `json:"created_at"`
  Id              *string `json:"id"`
  OwnerAddress    *string `json:"owner_address"`
  ReceiverAddress *string `json:"receiver_address"`
  WithdrawId      *int64  `json:"withdraw_id"`
}

type PublicRateUpdatesSelect struct {
  BlockNumber     int64  `json:"block_number"`
  BlockTimestamp  string `json:"block_timestamp"`
  ContractAddress string `json:"contract_address"`
  Id              string `json:"id"`
  Rate            string `json:"rate"`
}

type PublicRateUpdatesInsert struct {
  BlockNumber     int64   `json:"block_number"`
  BlockTimestamp  string  `json:"block_timestamp"`
  ContractAddress string  `json:"contract_address"`
  Id              *string `json:"id"`
  Rate            string  `json:"rate"`
}

type PublicRateUpdatesUpdate struct {
  BlockNumber     *int64  `json:"block_number"`
  BlockTimestamp  *string `json:"block_timestamp"`
  ContractAddress *string `json:"contract_address"`
  Id              *string `json:"id"`
  Rate            *string `json:"rate"`
}
