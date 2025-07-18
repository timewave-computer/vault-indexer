package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // Import PostgreSQL driver
	"github.com/timewave/vault-indexer/go-indexer/database"
)

/*
* Purpose: ingest historical data from scratch, and compare with remote records
* 1. load env vars from .env.scripts
* 2. fetch unique contract addresses from remote DB
* 2. for each contract + table, load all records and exec comparison function for table
 */

// Define a set of comparators
var comparators = []TableComparator{
	{
		TableName:   "events",
		CompareFn:   compareEvents,
		FetchString: "SELECT block_number, event_name, log_index, transaction_hash FROM events WHERE contract_address = $1 AND block_number <= $2 ORDER BY block_number ASC, log_index ASC",
		ScanRowsFn:  scanEventsRows,
	},
	{
		TableName:   "rate_updates",
		CompareFn:   compareRateUpdates,
		FetchString: "SELECT block_number, rate FROM rate_updates WHERE contract_address = $1 AND block_number <= $2 ORDER BY block_number ASC",
		ScanRowsFn:  scanRateUpdatesRows,
	},
	{
		TableName:   "withdraw_requests",
		CompareFn:   compareWithdrawRequests,
		FetchString: "SELECT amount, block_number, owner_address, receiver_address, withdraw_id FROM withdraw_requests WHERE contract_address = $1 AND block_number <= $2 ORDER BY block_number ASC, withdraw_id ASC",
		ScanRowsFn:  scanWithdrawRequestsRows,
	},
	{
		TableName:   "positions",
		CompareFn:   comparePositions,
		FetchString: "SELECT amount_shares, is_terminated, owner_address, position_end_height, position_index_id, position_start_height,withdraw_receiver_address FROM positions WHERE contract_address = $1 AND position_start_height <= $2 ORDER BY position_start_height ASC",
		ScanRowsFn:  scanPositionsRows,
	},
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load environment configuration

	cfg, err := loadEnvConfig()
	if err != nil {
		fmt.Printf("Error loading environment configuration: %v\n", err)
		os.Exit(1)
	}

	ethClient, localDB, remoteDB, err := setUpConnections(cfg)
	if err != nil {
		fmt.Printf("Error setting up connections: %v\n", err)
		os.Exit(1)
	}

	currentBlock, err := ethClient.BlockNumber(ctx)
	if err != nil {
		fmt.Printf("Error getting current block: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Current block: %d\n", currentBlock)

	contracts, err := getUniqueContractAddresses(remoteDB, currentBlock)
	if err != nil {
		fmt.Printf("Error getting unique contract addresses: %v\n", err)
		os.Exit(1)
	}
	errors := []error{}

	for _, comparator := range comparators {

		for _, contract := range contracts {
			fmt.Printf("Checking %s contract: %s\n", comparator.TableName, contract)
			_remoteRows, err := remoteDB.Query(comparator.FetchString, contract, currentBlock)
			if err != nil {
				fmt.Printf("Error fetching rows: %v\n", err)
				os.Exit(1)
			}
			remoteRows, err := comparator.ScanRowsFn(_remoteRows)
			if err != nil {
				fmt.Printf("Error scanning rows: %v\n", err)
				os.Exit(1)
			}
			_remoteRows.Close()
			_localRows, err := localDB.Query(comparator.FetchString, contract, currentBlock)
			if err != nil {
				fmt.Printf("Error fetching rows: %v\n", err)
				os.Exit(1)
			}
			localRows, err := comparator.ScanRowsFn(_localRows)
			if err != nil {
				fmt.Printf("Error scanning rows: %v\n", err)
				os.Exit(1)
			}
			_localRows.Close()

			err = comparator.CompareFn(localRows, remoteRows)
			if err != nil {
				errors = append(errors, err)
				fmt.Printf("ERROR: contract: %s table: %s %v\n", contract, comparator.TableName, err)
			}
		}
	}

	fmt.Printf("######### REPORT #########\n")

	if len(errors) > 0 {
		fmt.Printf("Found %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("ERROR: %v\n", err)
		}
		os.Exit(1)
	} else {
		fmt.Printf("Success:No errors found\n")
	}

}

func getUniqueContractAddresses(db *sql.DB, currentBlock uint64) ([]string, error) {
	contracts := []string{}

	rows, err := db.Query("SELECT DISTINCT contract_address from events where block_number <= $1", currentBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique contract addresses: %w", err)
	}

	var addr string
	for rows.Next() {
		err := rows.Scan(&addr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		contracts = append(contracts, addr)
	}

	defer rows.Close()

	return contracts, nil
}

func compareEvents(localRows, remoteRows any) error {
	l, ok1 := localRows.([]database.PublicEventsSelect)
	r, ok2 := remoteRows.([]database.PublicEventsSelect)

	if !ok1 || !ok2 {
		return fmt.Errorf("compareEvents: input types must be []database.PublicEventsSelect")
	}
	if len(l) != len(r) {
		return fmt.Errorf("row count mismatch: local=%d remote=%d", len(l), len(r))
	}
	for i := range l {
		l := l[i]
		r := r[i]
		if l.BlockNumber != r.BlockNumber || l.EventName != r.EventName || l.LogIndex != r.LogIndex || l.TransactionHash != r.TransactionHash {
			return fmt.Errorf("mismatch at index %d: LOCAL={block_number:%d event_name:%s log_index:%d transaction_hash:%s} REMOTE={block_number:%d event_name:%s log_index:%d transaction_hash:%s}",
				i, l.BlockNumber, l.EventName, l.LogIndex, l.TransactionHash, r.BlockNumber, r.EventName, r.LogIndex, r.TransactionHash)
		}
	}
	return nil
}

func compareRateUpdates(localRows, remoteRows any) error {
	l, ok1 := localRows.([]database.PublicRateUpdatesSelect)
	r, ok2 := remoteRows.([]database.PublicRateUpdatesSelect)

	if !ok1 || !ok2 {
		return fmt.Errorf("compareRateUpdates: input types must be []database.PublicRateUpdatesSelect")
	}
	if len(l) != len(r) {
		return fmt.Errorf("row count mismatch: local=%d remote=%d", len(l), len(r))
	}
	for i := range l {
		l := l[i]
		r := r[i]
		if l.Rate != r.Rate || l.BlockNumber != r.BlockNumber {
			return fmt.Errorf("mismatch at index %d: LOCAL={rate:%s block_number:%d} REMOTE={rate:%s block_number:%d}",
				i, l.Rate, l.BlockNumber, r.Rate, r.BlockNumber)
		}
	}
	return nil
}

func compareWithdrawRequests(localRows, remoteRows any) error {
	l, ok1 := localRows.([]database.PublicWithdrawRequestsSelect)
	r, ok2 := remoteRows.([]database.PublicWithdrawRequestsSelect)

	if !ok1 || !ok2 {
		return fmt.Errorf("compareWithdrawRequests: input types must be []database.PublicWithdrawRequestsSelect")
	}
	if len(l) != len(r) {
		return fmt.Errorf("row count mismatch: local=%d remote=%d", len(l), len(r))
	}
	for i := range l {
		l := l[i]
		r := r[i]
		if l.Amount != r.Amount || l.BlockNumber != r.BlockNumber || l.ReceiverAddress != r.ReceiverAddress || l.WithdrawId != r.WithdrawId {
			return fmt.Errorf("mismatch at index %d: LOCAL={amount:%s block_number:%d contract_address:%s id:%s receiver_address:%s withdraw_id:%d} REMOTE={amount:%s block_number:%d contract_address:%s id:%s receiver_address:%s withdraw_id:%d}",
				i, l.Amount, l.BlockNumber, l.ContractAddress, l.Id, l.ReceiverAddress, l.WithdrawId, r.Amount, r.BlockNumber, r.ContractAddress, r.Id, r.ReceiverAddress, r.WithdrawId)
		}
	}
	return nil
}

func comparePositions(localRows, remoteRows any) error {
	l, ok1 := localRows.([]database.PublicPositionsSelect)
	r, ok2 := remoteRows.([]database.PublicPositionsSelect)

	if !ok1 || !ok2 {
		return fmt.Errorf("comparePositions: input types must be []database.PublicPositionsSelect")
	}
	if len(l) != len(r) {
		return fmt.Errorf("row count mismatch: local=%d remote=%d", len(l), len(r))
	}
	for i := range l {
		l := l[i]
		r := r[i]

		// Helper function to safely compare pointer values
		compareInt64Ptr := func(a, b *int64) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return *a == *b
		}

		compareStringPtr := func(a, b *string) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return *a == *b
		}

		if l.AmountShares != r.AmountShares ||
			l.IsTerminated != r.IsTerminated ||
			l.OwnerAddress != r.OwnerAddress ||
			!compareInt64Ptr(l.PositionEndHeight, r.PositionEndHeight) ||
			l.PositionIndexId != r.PositionIndexId ||
			l.PositionStartHeight != r.PositionStartHeight ||
			!compareStringPtr(l.WithdrawReceiverAddress, r.WithdrawReceiverAddress) {

			// Format values for error message
			lEndHeight := "nil"
			if l.PositionEndHeight != nil {
				lEndHeight = fmt.Sprintf("%d", *l.PositionEndHeight)
			}
			rEndHeight := "nil"
			if r.PositionEndHeight != nil {
				rEndHeight = fmt.Sprintf("%d", *r.PositionEndHeight)
			}

			lWithdrawAddr := "nil"
			if l.WithdrawReceiverAddress != nil {
				lWithdrawAddr = *l.WithdrawReceiverAddress
			}
			rWithdrawAddr := "nil"
			if r.WithdrawReceiverAddress != nil {
				rWithdrawAddr = *r.WithdrawReceiverAddress
			}

			return fmt.Errorf("mismatch sat index %d: LOCAL={amount_shares:%s is_terminated:%t owner_address:%s position_end_height:%s position_index_id:%d position_start_height:%d withdraw_receiver_address:%s} REMOTE={amount_shares:%s is_terminated:%t owner_address:%s position_end_height:%s position_index_id:%d position_start_height:%d withdraw_receiver_address:%s}",
				i, l.AmountShares, l.IsTerminated, l.OwnerAddress, lEndHeight, l.PositionIndexId, l.PositionStartHeight, lWithdrawAddr, r.AmountShares, r.IsTerminated, r.OwnerAddress, rEndHeight, r.PositionIndexId, r.PositionStartHeight, rWithdrawAddr)
		}
	}
	return nil
}

func setUpConnections(cfg *EnvConfig) (*ethclient.Client, *sql.DB, *sql.DB, error) {
	ethClient, err := ethclient.Dial(cfg.EthereumWebsocketURL)
	if err != nil {
		fmt.Printf("Error connecting to Ethereum: %v\n", err)
		os.Exit(1)
	}

	localDB, err := sql.Open("postgres", cfg.LocalPGConnectionString)
	if err != nil {
		fmt.Printf("Error connecting to local DB: %v\n", err)
		os.Exit(1)
	}

	remoteDB, err := sql.Open("postgres", cfg.RemotePGConnectionString)
	if err != nil {
		fmt.Printf("Error connecting to remote DB: %v\n", err)
		os.Exit(1)
	}

	return ethClient, localDB, remoteDB, nil
}

// EnvConfig holds the environment configuration
type EnvConfig struct {
	EthereumWebsocketURL     string
	LocalPGConnectionString  string
	RemotePGConnectionString string
}

// loadEnvConfig loads environment variables from .env file and returns the configuration
func loadEnvConfig() (*EnvConfig, error) {
	// Get the directory of the current script
	scriptDir, err := os.Getwd()
	fmt.Printf("Current directory: %s\n", scriptDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Look for .env file in the same directory as the script
	envPath := filepath.Join(scriptDir, ".env.scripts")

	fmt.Printf("Loading environment variables from %s\n", envPath)
	// Try to load .env.scripts file if it exists
	if fileInfo, err := os.Stat(envPath); err == nil {
		fmt.Printf("Env file exists: %s\n", fileInfo.Name())
		if err := godotenv.Load(envPath); err != nil {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	} else {
		fmt.Printf("Env file does not exist: %s\n", envPath)
	}

	// Extract the required environment variables
	config := &EnvConfig{
		EthereumWebsocketURL:     os.Getenv("ETHEREUM_WEBSOCKET_URL"),
		LocalPGConnectionString:  os.Getenv("LOCAL_PG_CONNECTION_STRING"),
		RemotePGConnectionString: os.Getenv("REMOTE_PG_CONNECTION_STRING"),
	}

	// Validate that all required environment variables are present
	if config.EthereumWebsocketURL == "" {
		return nil, fmt.Errorf("ETHEREUM_WEBSOCKET_URL environment variable is required")
	}
	if config.LocalPGConnectionString == "" {
		return nil, fmt.Errorf("LOCAL_PG_CONNECTION_STRING environment variable is required")
	}
	if config.RemotePGConnectionString == "" {
		return nil, fmt.Errorf("REMOTE_PG_CONNECTION_STRING environment variable is required")
	}

	return config, nil
}

// TableComparator holds the table name and a comparison function
// CompareFn takes two slices of any type (localRows, remoteRows) and returns an error if comparison fails
type TableComparator struct {
	TableName   string
	CompareFn   func(localRows, remoteRows any) error
	FetchString string
	ScanRowsFn  func(*sql.Rows) (any, error)
}

func scanEventsRows(rows *sql.Rows) (any, error) {
	var results []database.PublicEventsSelect
	for rows.Next() {
		var r database.PublicEventsSelect
		err := rows.Scan(
			&r.BlockNumber,
			&r.EventName,
			&r.LogIndex,
			&r.TransactionHash,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// scanRateUpdatesRows scans sql.Rows into []database.PublicRateUpdatesSelect
func scanRateUpdatesRows(rows *sql.Rows) (any, error) {
	var results []database.PublicRateUpdatesSelect
	for rows.Next() {
		var r database.PublicRateUpdatesSelect
		err := rows.Scan(
			&r.BlockNumber,
			&r.Rate,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func scanWithdrawRequestsRows(rows *sql.Rows) (any, error) {
	var results []database.PublicWithdrawRequestsSelect
	for rows.Next() {
		var r database.PublicWithdrawRequestsSelect
		err := rows.Scan(
			&r.Amount,
			&r.BlockNumber,
			&r.OwnerAddress,
			&r.ReceiverAddress,
			&r.WithdrawId,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func scanPositionsRows(rows *sql.Rows) (any, error) {
	var results []database.PublicPositionsSelect
	for rows.Next() {
		var r database.PublicPositionsSelect

		// Use sql.NullInt64 for nullable fields
		var positionEndHeight sql.NullInt64
		var withdrawReceiverAddress sql.NullString

		err := rows.Scan(
			&r.AmountShares,
			&r.IsTerminated,
			&r.OwnerAddress,
			&positionEndHeight,
			&r.PositionIndexId,
			&r.PositionStartHeight,
			&withdrawReceiverAddress,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.NullInt64 to *int64
		if positionEndHeight.Valid {
			r.PositionEndHeight = &positionEndHeight.Int64
		} else {
			r.PositionEndHeight = nil
		}

		// Convert sql.NullString to *string
		if withdrawReceiverAddress.Valid {
			r.WithdrawReceiverAddress = &withdrawReceiverAddress.String
		} else {
			r.WithdrawReceiverAddress = nil
		}

		results = append(results, r)
	}
	return results, rows.Err()
}
