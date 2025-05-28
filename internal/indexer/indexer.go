package indexer

import (
	"context"
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

// Indexer handles blockchain event indexing and position tracking
type Indexer struct {
	config            *config.Config
	client            *ethclient.Client
	db                *supa.Client
	ctx               context.Context
	cancel            context.CancelFunc
	positionChan      chan PositionEvent
	positionProcessor *PositionProcessor
	eventProcessor    *EventProcessor
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

	// Create processors
	positionProcessor := NewPositionProcessor(db)
	eventProcessor := NewEventProcessor(db, client)
	positionChan := make(chan PositionEvent, 1000) // Buffer size of 1000 events

	return &Indexer{
		config:            cfg,
		client:            client,
		db:                db,
		ctx:               ctx,
		cancel:            cancel,
		positionChan:      positionChan,
		positionProcessor: positionProcessor,
		eventProcessor:    eventProcessor,
	}, nil
}

func (i *Indexer) Start() error {
	log.Println("Starting indexer...")

	// Start position processor
	i.positionProcessor.Start(i.positionChan)

	// Perform health check by getting current block height
	blockNumber, err := i.client.BlockNumber(i.ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	log.Printf("Health check successful - Current block height: %d", blockNumber)

	// Process historical events for each contract
	for _, contract := range i.config.Contracts {
		if err := i.processHistoricalEvents(contract); err != nil {
			return fmt.Errorf("failed to process historical events for contract %s: %w", contract.Name, err)
		}
	}

	// Set up subscriptions for new events
	for _, contract := range i.config.Contracts {
		if err := i.setupEventSubscriptions(contract); err != nil {
			return fmt.Errorf("failed to set up event subscriptions for contract %s: %w", contract.Name, err)
		}
	}

	return nil
}

// processHistoricalEvents processes all historical events for a contract
func (i *Indexer) processHistoricalEvents(contract config.ContractConfig) error {
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

	return nil
}

// setupEventSubscriptions sets up subscriptions for new events
func (i *Indexer) setupEventSubscriptions(contract config.ContractConfig) error {
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
	// Process the event using the event processor
	eventData, err := i.eventProcessor.ProcessEvent(vLog, event, contractName)
	if err != nil {
		return fmt.Errorf("failed to process event: %w", err)
	}

	// Send event to position processor if it's a position-related event
	if event.Name == "Deposit" || event.Name == "Withdraw" || event.Name == "Transfer" {
		select {
		case i.positionChan <- PositionEvent{
			EventName: event.Name,
			EventData: eventData,
			Log:       vLog,
		}:
		case <-i.ctx.Done():
			return fmt.Errorf("context cancelled while sending event to position processor")
		}
	}

	return nil
}

func (i *Indexer) Stop() error {
	log.Println("Stopping indexer...")
	i.cancel()
	i.client.Close()
	i.positionProcessor.Stop()
	i.eventProcessor.Stop()
	close(i.positionChan)
	return nil
}
