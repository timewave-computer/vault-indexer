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

	return nil
}

func (i *Indexer) Stop() error {
	log.Println("Stopping indexer...")
	i.cancel()
	i.client.Close()
	// Supabase client doesn't need explicit cleanup
	return nil
}
