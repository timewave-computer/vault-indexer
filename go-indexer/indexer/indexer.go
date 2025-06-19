package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/config"
	"github.com/timewave/vault-indexer/go-indexer/health"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// Indexer handles blockchain event indexing and position tracking
type Indexer struct {
	config         *config.Config
	ethClient      *ethclient.Client
	supabaseClient *supa.Client
	ctx            context.Context
	cancel         context.CancelFunc
	postgresClient *sql.DB
	eventProcessor *EventProcessor
	transformer    *Transformer
	wg             sync.WaitGroup
	logger         *logger.Logger
	healthServer   *health.Server
}

func New(ctx context.Context, cfg *config.Config) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	indexerLogger := logger.NewLogger("Indexer")

	indexerLogger.Info("Starting indexer...")

	ethClient, err := ethclient.Dial(cfg.Ethereum.WebsocketURL)
	indexerLogger.Info("Dialed to Ethereum websocket")
	if err != nil {
		cancel()
		return nil, err
	}
	indexerLogger.Info("Connected to Ethereum websocket")

	// Connect to Supabase
	supabaseClient, err := supa.NewClient(cfg.Database.SupabaseURL, cfg.Database.SupabaseKey, &supa.ClientOptions{})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Supabase: %w", err)
	}
	indexerLogger.Info("Connected to supabase client")

	// Create processors
	eventProcessor := NewEventProcessor(supabaseClient, ethClient)

	postgresClient, err := sql.Open("postgres", cfg.Database.PostgresConnectionString)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Postgres: %w", err)
	}
	indexerLogger.Info("Connected to pgdb")

	transformer, err := NewTransformer(supabaseClient, postgresClient, ethClient)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create transformer: %w", err)
	}

	// Create health check server
	healthServer := health.NewServer(8080, "event_ingestion") // Default port for indexer health checks

	return &Indexer{
		config:         cfg,
		postgresClient: postgresClient,
		ethClient:      ethClient,
		supabaseClient: supabaseClient,
		ctx:            ctx,
		cancel:         cancel,
		eventProcessor: eventProcessor,
		transformer:    transformer,
		logger:         indexerLogger,
		healthServer:   healthServer,
	}, nil
}

func (i *Indexer) Start() error {
	i.logger.Info("Starting indexer...")

	// Start health check server
	if err := i.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	// Perform health check by getting current block height
	blockNumber, err := i.ethClient.BlockNumber(i.ctx)
	if err != nil {
		i.healthServer.SetStatus("unhealthy")

		_ = i.healthServer.Stop() // best-effort cleanup
		return fmt.Errorf("health check failed: %w", err)

	}
	i.healthServer.SetStatus("healthy")
	i.logger.Info("Health check successful - Current block height: %d", blockNumber)

	i.logger.Info("Processing historical events for %d contracts...", len(i.config.Contracts))

	// Process historical events for each contract
	for _, contract := range i.config.Contracts {
		// will read historical events from the database and record them
		if err := i.loadHistoricalEvents(contract); err != nil {
			return fmt.Errorf("failed to process historical events for contract %s: %w", contract.Name, err)
		}
	}

	i.logger.Info("Listening to event subscriptions for %d contracts...", len(i.config.Contracts))
	// set up event subscriptions for all contracts
	for _, contract := range i.config.Contracts {
		if err := i.setupEventSubscriptions(contract); err != nil {
			// will subscribe to events for all contracts and begin recording them
			return fmt.Errorf("failed to set up event subscriptions for contract %s: %w", contract.Name, err)
		}
	}

	// Start transformer after historical ingestion
	if err := i.transformer.Start(); err != nil {
		return fmt.Errorf("failed to start transformer: %w", err)
	}

	go func() {
		<-i.transformer.ctx.Done()
		i.transformer.Stop()
	}()

	return nil
}

// processHistoricalEvents processes all historical events for a contract
func (i *Indexer) loadHistoricalEvents(contract config.ContractConfig) error {
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
	currentBlock, err := i.ethClient.BlockNumber(i.ctx)
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

		logs, err := i.ethClient.FilterLogs(i.ctx, query)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		// Process each log
		for _, vLog := range logs {
			if err := i.eventProcessor.processEvent(vLog, event); err != nil {
				return fmt.Errorf("failed to process event: %w", err)
			}
		}
	}

	return nil
}

func (i *Indexer) setupEventSubscriptions(contract config.ContractConfig) error {
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

	for _, eventName := range contract.Events {
		event := parsedABI.Events[eventName]

		i.wg.Add(1)
		go func(event abi.Event) {
			defer i.wg.Done()
			for {
				err := i.subscribeToEvent(contractAddress, event)
				i.logger.Error("Subscription error: %v", err)

				// Try reconnecting to eth client
				if reconnectErr := i.reconnectEthClient(); reconnectErr != nil {
					i.logger.Error("Could not reconnect. Stopping indexer.")
					i.cancel()
					return
				}

				time.Sleep(2 * time.Second) // Brief pause before retrying subscription
				continue

			}
		}(event)
	}

	return nil
}

func (i *Indexer) Stop() error {
	i.logger.Info("Stopping indexer...")

	// Stop health check server
	if err := i.healthServer.Stop(); err != nil {
		i.logger.Error("Error stopping health check server: %v", err)
	}

	// signal writers to stop
	i.cancel()

	// wait until all goroutines finish
	i.wg.Wait()

	i.eventProcessor.Stop()
	i.transformer.Stop()

	// finally release external resources
	i.ethClient.Close()
	i.postgresClient.Close()

	return nil
}

func (i *Indexer) reconnectEthClient() error {
	i.logger.Info("Reconnecting to Ethereum websocket...")

	var err error
	for attempt := 1; attempt <= 5; attempt++ {
		i.ethClient.Close() // Close old connection

		i.ethClient, err = ethclient.Dial(i.config.Ethereum.WebsocketURL)
		if err == nil {
			i.logger.Info("Reconnected to Ethereum websocket successfully.")
			return nil
		}

		i.logger.Error("Reconnect attempt %d failed: %v", attempt, err)
		time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
	}

	return fmt.Errorf("failed to reconnect after several attempts: %w", err)
}

func (i *Indexer) subscribeToEvent(contractAddress common.Address, event abi.Event) error {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{event.ID}},
	}

	logs := make(chan types.Log)
	sub, err := i.ethClient.SubscribeFilterLogs(i.ctx, query, logs)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %w", err)
	}

	for {
		select {
		case err := <-sub.Err():
			i.logger.Error("Subscription error: %v", err)
			sub.Unsubscribe()
			return err // Bubble up the error so outer loop can reconnect

		case vLog := <-logs:
			if err := i.eventProcessor.processEvent(vLog, event); err != nil {
				i.logger.Error("Error processing event: %v", err)
			}
		case <-i.ctx.Done():
			sub.Unsubscribe()
			return nil
		}
	}
}
