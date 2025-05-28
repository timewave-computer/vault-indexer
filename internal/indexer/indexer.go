package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/internal/config"
)

type Indexer struct {
	config *config.Config
	client *ethclient.Client
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg *config.Config) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client, err := ethclient.Dial(cfg.Ethereum.WebsocketURL)
	if err != nil {
		cancel()
		return nil, err
	}

	// Connect to Supabase
	db, err := supa.NewClient(cfg.Database.SupabaseURL, cfg.Database.SupabaseKey, &supa.ClientOptions{})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Supabase: %w", err)
	}

	return &Indexer{
		config: cfg,
		client: client,
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (i *Indexer) Start() error {
	log.Println("Starting indexer...")

	// Process historical events

	// Perform health check by getting current block height
	blockNumber, err := i.client.BlockNumber(i.ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	log.Printf("Health check successful - Current block height: %d", blockNumber)

	// Process each contract
	for _, contract := range i.config.Contracts {
		if err := i.processContract(contract); err != nil {
			return fmt.Errorf("failed to process contract %s: %w", contract.Name, err)
		}
	}

	// TODO: subscribe to new events

	return nil
}

func (i *Indexer) processContract(contract config.ContractConfig) error {
	// Load ABI
	abiPath := filepath.Join("abis", contract.ABIPath)
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return fmt.Errorf("failed to read ABI file: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return fmt.Errorf("failed to parse ABI: %w", err)
	}

	contractAddress := common.HexToAddress(contract.Address)

	// Get current block number
	currentBlock, err := i.client.BlockNumber(i.ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	// Process historical events
	fromBlock := big.NewInt(int64(contract.StartBlock))
	toBlock := big.NewInt(int64(currentBlock))

	// Create filter query for each event
	for _, eventName := range contract.Events {
		event, exists := parsedABI.Events[eventName]
		if !exists {
			return fmt.Errorf("event %s not found in ABI", eventName)
		}

		query := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{contractAddress},
			Topics:    [][]common.Hash{{event.ID}},
		}

		logs, err := i.client.FilterLogs(i.ctx, query)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		// Process each log
		for _, vLog := range logs {
			if err := i.processEvent(vLog, event, contract.Name); err != nil {
				return fmt.Errorf("failed to process event: %w", err)
			}
		}
	}

	// Set up event subscription for new events
	for _, eventName := range contract.Events {
		event := parsedABI.Events[eventName]
		query := ethereum.FilterQuery{
			Addresses: []common.Address{contractAddress},
			Topics:    [][]common.Hash{{event.ID}},
		}

		logs := make(chan types.Log)
		sub, err := i.client.SubscribeFilterLogs(i.ctx, query, logs)
		if err != nil {
			return fmt.Errorf("failed to subscribe to logs: %w", err)
		}

		go func() {
			for {
				select {
				case err := <-sub.Err():
					log.Printf("Subscription error: %v", err)
					return
				case vLog := <-logs:
					if err := i.processEvent(vLog, event, contract.Name); err != nil {
						log.Printf("Error processing event: %v", err)
					}
				case <-i.ctx.Done():
					return
				}
			}
		}()
	}

	return nil
}

func (i *Indexer) processEvent(vLog types.Log, event abi.Event, contractName string) error {
	// Parse the event data
	eventData := make(map[string]interface{})
	if err := event.Inputs.UnpackIntoMap(eventData, vLog.Data); err != nil {
		return fmt.Errorf("failed to unpack event data: %w", err)
	}

	// Convert event data to JSON for storage
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Create the event record matching the database schema
	eventRecord := map[string]interface{}{
		"contract_address": vLog.Address.Hex(),
		"event_name":       event.Name,
		"block_number":     vLog.BlockNumber,
		"transaction_hash": vLog.TxHash.Hex(),
		"log_index":        vLog.Index,
		"raw_data":         string(eventJSON),
	}
	log.Printf("Event record: %v", eventRecord)

	// Insert into Supabase
	_, _, err = i.db.From("events").Insert(eventRecord, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to insert event into database: %w", err)
	}
	log.Printf("Inserted event into Supabase: %v", eventRecord)

	// Transform event to position if applicable
	if err := i.transformEventToPosition(event.Name, eventData, vLog); err != nil {
		log.Printf("Warning: failed to transform event to position: %v", err)
	}

	return nil
}

func (i *Indexer) transformEventToPosition(eventName string, eventData map[string]interface{}, vLog types.Log) error {
	var accountAddress string
	var amount float64
	var entryMethod string
	var exitMethod string

	// Handle different event types
	switch eventName {
	case "Deposit":
		if owner, ok := eventData["owner"].(common.Address); ok {
			accountAddress = owner.Hex()
		}
		if assets, ok := eventData["assets"].(*big.Int); ok {
			amount = float64(assets.Int64())
		}
		entryMethod = "deposit"
	case "Withdraw":
		if owner, ok := eventData["owner"].(common.Address); ok {
			accountAddress = owner.Hex()
		}
		if assets, ok := eventData["assets"].(*big.Int); ok {
			amount = -float64(assets.Int64())
		}
		exitMethod = "withdraw"
	case "Transfer":
		if to, ok := eventData["to"].(common.Address); ok {
			accountAddress = to.Hex()
		}
		if value, ok := eventData["value"].(*big.Int); ok {
			amount = float64(value.Int64())
		}
		entryMethod = "transfer"
	default:
		return nil
	}

	if accountAddress == "" {
		return fmt.Errorf("could not determine account address from event data")
	}

	// Get current position if it exists
	var currentPosition struct {
		PositionIndexNumber int64   `json:"position_index_number"`
		Amount              float64 `json:"amount"`
		PositionEndHeight   *int64  `json:"position_end_height"`
	}
	_, _, err := i.db.From("positions").
		Select("position_index_number,amount,position_end_height", "", false).
		Eq("account_address", accountAddress).
		Eq("contract_address", vLog.Address.Hex()).
		Is("position_end_height", "null").
		Single().
		Execute()

	// Calculate new amount
	newAmount := amount
	if err == nil {
		newAmount += currentPosition.Amount
	}

	// Close current position if it exists
	if err == nil {
		endHeight := int64(vLog.BlockNumber - 1)
		_, _, err = i.db.From("positions").
			Update(map[string]interface{}{
				"position_end_height": endHeight,
				"exit_method":         exitMethod,
				"is_terminated":       newAmount == 0,
			}, "", "").
			Eq("position_index_number", fmt.Sprintf("%d", currentPosition.PositionIndexNumber)).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to close current position: %w", err)
		}
	}

	// Create new position if amount is not zero
	if newAmount != 0 {
		// Get the highest position index
		var maxPosition struct {
			MaxIndex int64 `json:"max_index"`
		}
		_, _, err = i.db.From("positions").
			Select("COALESCE(MAX(position_index_number), 0) as max_index", "", false).
			Single().
			Execute()
		if err != nil {
			return fmt.Errorf("failed to get max position index: %w", err)
		}

		positionIndex := maxPosition.MaxIndex + 1

		positionRecord := map[string]interface{}{
			"position_index_number": positionIndex,
			"account_address":       accountAddress,
			"contract_address":      vLog.Address.Hex(),
			"amount":                newAmount,
			"position_start_height": vLog.BlockNumber,
			"position_end_height":   nil,
			"entry_method":          entryMethod,
			"exit_method":           nil,
			"is_terminated":         false,
			"neutron_address":       nil, // TODO: Add neutron address handling when available
		}

		_, _, err = i.db.From("positions").Insert(positionRecord, false, "", "", "").Execute()
		if err != nil {
			return fmt.Errorf("failed to create new position: %w", err)
		}

		log.Printf("Created new position for account %s: amount = %f, index = %d", accountAddress, newAmount, positionIndex)
	}

	return nil
}

func (i *Indexer) Stop() error {
	log.Println("Stopping indexer...")
	i.cancel()
	i.client.Close()
	// Supabase client doesn't need explicit cleanup
	return nil
}
