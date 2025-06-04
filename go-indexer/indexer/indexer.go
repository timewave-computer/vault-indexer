package indexer

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/config"
)

// Indexer handles blockchain event indexing and position tracking
type Indexer struct {
	config            *config.Config
	client            *ethclient.Client
	db                *supa.Client
	ctx               context.Context
	cancel            context.CancelFunc
	positionChan      chan PositionEvent
	withdrawChan      chan WithdrawRequestEvent
	positionProcessor *PositionProcessor
	withdrawProcessor *WithdrawProcessor
	eventProcessor    *EventProcessor
	wg                sync.WaitGroup
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
	eventProcessor := NewEventProcessor(db, client)
	positionChan := make(chan PositionEvent, 1000)        // Buffer size of 1000 events
	withdrawChan := make(chan WithdrawRequestEvent, 1000) // Buffer size of 1000 events
	positionProcessor := NewPositionProcessor(db)
	withdrawProcessor := NewWithdrawProcessor(db)

	return &Indexer{
		config:            cfg,
		client:            client,
		db:                db,
		ctx:               ctx,
		cancel:            cancel,
		positionChan:      positionChan,
		withdrawChan:      withdrawChan,
		positionProcessor: positionProcessor,
		withdrawProcessor: withdrawProcessor,
		eventProcessor:    eventProcessor,
	}, nil
}

func (i *Indexer) Start() error {
	log.Println("Starting indexer...")

	// Start position processor
	if err := i.positionProcessor.Start(i.positionChan); err != nil {
		return fmt.Errorf("failed to start position processor: %w", err)
	}

	// Start withdraw processor
	if err := i.withdrawProcessor.Start(i.withdrawChan); err != nil {
		return fmt.Errorf("failed to start withdraw processor: %w", err)
	}

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

	// TODO: there is a window here where there might be new events that are missed while processing historical events.
	// add a check to see if there are any new events since the last time we processed historical events.
	// OR set up subscriptions immediately so the queue can fill, but only process them after historical events are processed.

	// AFTER all historical events have been processed, set up subscriptions for new events
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
	var toBlock *big.Int
	if contract.EndBlock == 0 {
		toBlock = big.NewInt(int64(currentBlock))
	} else {
		toBlock = big.NewInt(int64(contract.EndBlock))
	}

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

		i.wg.Add(1)
		go func() {
			defer i.wg.Done() // decrements waitgroup after goroutine finishes
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

	// Send event to appropriate processor based on event type
	switch event.Name {
	case "WithdrawRequested":
		// Send to position channel
		select {
		case i.positionChan <- PositionEvent{
			EventName: event.Name,
			EventData: eventData,
			Log:       vLog,
		}:
		case <-i.ctx.Done():
			return fmt.Errorf("context cancelled while sending withdraw requested event to position processor")
		}
		// Send to withdraw channel
		select {
		case i.withdrawChan <- WithdrawRequestEvent{
			EventName: event.Name,
			EventData: eventData,
			Log:       vLog,
		}:
		case <-i.ctx.Done():
			return fmt.Errorf("context cancelled while sending withdraw requested event to withdraw processor")
		}

	case "Transfer":
		select {
		case i.positionChan <- PositionEvent{
			EventName: event.Name,
			EventData: eventData,
			Log:       vLog,
		}:
		case <-i.ctx.Done():
			return fmt.Errorf("context cancelled while sending transfer event to position processor")
		}
	}
	return nil
}

func (i *Indexer) Stop() error {
	log.Println("Stopping indexer...")

	// signal writers to stop
	i.cancel()

	// close channels so the reader goroutines can exit gracefully
	close(i.positionChan)
	close(i.withdrawChan)

	// wait until all goroutines finish
	i.wg.Wait()

	// stop processors
	i.positionProcessor.Stop()
	i.withdrawProcessor.Stop()
	i.eventProcessor.Stop()

	// finally release external resources
	i.client.Close()
	return nil
}
