package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/config"
	"github.com/timewave/vault-indexer/go-indexer/database"
	event_queue "github.com/timewave/vault-indexer/go-indexer/event-ingestion-queue"
	"github.com/timewave/vault-indexer/go-indexer/health"
	"github.com/timewave/vault-indexer/go-indexer/logger"
	"github.com/timewave/vault-indexer/go-indexer/reorg"
)

// Indexer handles blockchain event indexing and position tracking
type Indexer struct {
	config                *config.Config
	ethClient             *ethclient.Client
	supabaseClient        *supa.Client
	ctx                   context.Context
	cancel                context.CancelFunc
	postgresClient        *sql.DB
	extractor             *Extractor
	transformer           *Transformer
	logger                *logger.Logger
	healthServer          *health.Server
	eventQueue            *event_queue.EventQueue
	requiredConfirmations uint64
	finalityProcessor     *FinalityProcessor
	reorgHandler          *reorg.Handler
	once                  sync.Once
	errorChan             chan error
	stopChan              chan struct{}
	processWg             sync.WaitGroup
	eventProcessor        *EventProcessor
}

func New(ctx context.Context, cfg *config.Config) (*Indexer, error) {
	ctx, cancel := context.WithCancel(ctx)
	indexerLogger := logger.NewLogger("Indexer")

	ethClient, err := ethclient.Dial(cfg.Ethereum.WebsocketURL)
	indexerLogger.Info("Dialed to Ethereum websocket")
	if err != nil {
		cancel()
		return nil, err
	}
	indexerLogger.Info("Connected to Ethereum websocket")

	supabaseClient, err := supa.NewClient(cfg.Database.SupabaseURL, cfg.Database.SupabaseKey, &supa.ClientOptions{})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Supabase: %w", err)
	}
	indexerLogger.Info("Connected to supabase client")

	extractor := NewExtractor(supabaseClient, ethClient)

	eventQueue := event_queue.NewEventQueue()

	eventProcessor := NewEventProcessor(eventQueue, extractor, ethClient, 4)

	errorChan := make(chan error)

	finalityProcessor := NewFinalityProcessor(ethClient, supabaseClient, errorChan)

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

	// Create reorg handler
	reorgHandler := reorg.NewHandler(supabaseClient, postgresClient, ethClient, cfg)

	// Create health check server
	healthServer := health.NewServer(8080, "event_ingestion") // Default port for indexer health checks

	return &Indexer{
		config:                cfg,
		postgresClient:        postgresClient,
		ethClient:             ethClient,
		supabaseClient:        supabaseClient,
		ctx:                   ctx,
		cancel:                cancel,
		extractor:             extractor,
		transformer:           transformer,
		logger:                indexerLogger,
		healthServer:          healthServer,
		finalityProcessor:     finalityProcessor,
		reorgHandler:          reorgHandler,
		eventQueue:            eventQueue,
		requiredConfirmations: 4,
		errorChan:             errorChan,
		stopChan:              make(chan struct{}),
		processWg:             sync.WaitGroup{},
		eventProcessor:        eventProcessor,
	}, nil
}

func (i *Indexer) Start() error {
	i.logger.Info("Indexer started")

	// Start centralized error handler
	i.startErrorHandler()

	if err := i.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	if err := i.finalityProcessor.Start(); err != nil {
		return fmt.Errorf("failed to start finality processor: %w", err)
	}

	currentBlock, err := i.ethClient.BlockNumber(i.ctx)
	if err != nil {
		i.Stop()
		return fmt.Errorf("failed to get current block height: %w", err)
	}
	i.logger.Info("Current block height: %d", currentBlock)

	/*
		1. Start subscription and writing events immediately
	*/
	i.logger.Info("Creating subscriptions for %d contracts...", len(i.config.Contracts))

	wg := sync.WaitGroup{}
	err = i.setupSubscriptions(&wg)
	wg.Wait() // do not progress until all subscriptions are created
	if err != nil {
		return fmt.Errorf("failed to setup subscriptions: %w", err)
	}
	i.logger.Info("Subscriptions created")

	/*
		2. Backfill missed events
	*/

	i.logger.Info("Loading historical events, current block: %d", currentBlock)
	err = i.loadHistoricalEvents(currentBlock, &wg)
	wg.Wait() // do not progress until all historical events are loaded
	if err != nil {
		return fmt.Errorf("failed to backfill events: %w", err)
	}

	i.logger.Info("Loaded historical events into ingestion queue. Size: %d", i.eventQueue.Len())

	/*
		4. Start processor (process events from sorted queue)
	*/
	err = i.eventProcessor.Start()
	if err != nil {
		return fmt.Errorf("event queue processor failure: %w", err)
	}

	/*
		5. Start transformer (process events saved in database)
	*/

	if err := i.transformer.Start(); err != nil {
		return fmt.Errorf("failed to start transformer: %w", err)
	}

	// Monitor child processes and stop indexer if any of them stop
	i.processWg.Add(1)
	go func() {
		defer i.processWg.Done()
		select {
		case <-i.eventProcessor.ctx.Done():
			i.logger.Warn("Event processor context cancelled, stopping indexer")
			i.errorChan <- fmt.Errorf("event processor stopped")
		case <-i.stopChan:
			return
		}
	}()

	i.processWg.Add(1)
	go func() {
		defer i.processWg.Done()
		select {
		case <-i.transformer.ctx.Done():
			i.logger.Warn("Transformer context cancelled, stopping indexer")
			i.errorChan <- fmt.Errorf("transformer stopped")
		case <-i.stopChan:
			return
		}
	}()

	i.processWg.Add(1)
	go func() {
		defer i.processWg.Done()
		select {
		case <-i.finalityProcessor.ctx.Done():
			i.logger.Warn("Finality processor context cancelled, stopping indexer")
			i.errorChan <- fmt.Errorf("finality processor stopped")
		case <-i.stopChan:
			return
		}
	}()

	// Wait for context cancellation or stop signal
	select {
	case <-i.ctx.Done():
		i.logger.Info("Context cancelled, shutting down indexer...")
	case <-i.stopChan:
		i.logger.Info("Stop signal received, shutting down indexer...")
	}

	return nil
}

func (i *Indexer) setupSubscriptions(wg *sync.WaitGroup) error {

	for _, contract := range i.config.Contracts {
		parsedABI, err := i.loadAbi(contract)
		if err != nil {
			return fmt.Errorf("failed to load ABI for contract %s: %w", contract.Address, err)
		}
		contractAddress := common.HexToAddress(contract.Address)

		for _, _eventName := range contract.Events {
			eventName := parsedABI.Events[_eventName]

			wg.Add(1)
			go func(event abi.Event) {
				// Signal that this subscription goroutine has started
				wg.Done()

				for {
					// Check if context is cancelled before attempting subscription
					select {
					case <-i.ctx.Done():
						// Don't send error on context cancellation during shutdown
						return
					default:
						// Continue with subscription
					}

					err := i.subscribeToEvent(contractAddress, event)

					if err != nil {
						// Use centralized error channel
						select {
						case i.errorChan <- fmt.Errorf("failed to subscribe to event %s for contract %s: %w", event.Name, contract.Address, err):
						default:
							// Channel is closed or full, ignore
						}
						return
					}

					// Check context again before sleeping
					select {
					case <-i.ctx.Done():
						// Don't send error on context cancellation during shutdown
						return
					case <-time.After(2 * time.Second):
						// Continue to next iteration
					}
				}
			}(eventName)
		}
	}

	return nil
}

func (i *Indexer) loadHistoricalEvents(_currentBlock uint64, wg *sync.WaitGroup) error {

	for _, contractConfig := range i.config.Contracts {

		for _, eventName := range contractConfig.Events {
			wg.Add(1)
			parsedABI, err := i.loadAbi(contractConfig)
			if err != nil {
				return fmt.Errorf("failed to load ABI for contract %s: %w", contractConfig.Address, err)
			}
			event := parsedABI.Events[eventName]

			go func(contractConfig config.ContractConfig, event abi.Event) {
				defer wg.Done()
				contractAddress := common.HexToAddress(contractConfig.Address)

				select {
				case <-i.ctx.Done():
					return
				default:
					// continue
				}
				lastIndexedBlock, err := i.getLastIndexedBlock(contractAddress, event)
				if err != nil {
					// Use centralized error channel
					select {
					case i.errorChan <- fmt.Errorf("failed to get last indexed block: %w, event: %s, contract: %s", err, event.Name, contractConfig.Address):
					default:
						// Channel is closed or full, ignore
					}
					return
				}
				if lastIndexedBlock == nil {
					v := int64(contractConfig.StartBlock)
					lastIndexedBlock = &v
				}

				fromBlock := big.NewInt(int64(*lastIndexedBlock))
				toBlock := big.NewInt(int64(_currentBlock))

				i.logger.Info("Fetching historical events for %s on contract %s, from block %d to block %d", event.Name, contractConfig.Address, *lastIndexedBlock, _currentBlock)

				query := ethereum.FilterQuery{
					Addresses: []common.Address{contractAddress},
					Topics:    [][]common.Hash{{event.ID}},
					FromBlock: fromBlock,
					ToBlock:   toBlock,
				}

				logs, err := i.ethClient.FilterLogs(i.ctx, query)
				if err != nil {
					// Use centralized error channel
					select {
					case i.errorChan <- fmt.Errorf("failed to get historical events: %w, event: %s, contract: %s", err, event.Name, contractConfig.Address):
					default:
						// Channel is closed or full, ignore
					}
					return
				}

				for _, vLog := range logs {
					i.logger.Debug("Received historical event: %v", vLog)
					eventLog := event_queue.EventLog{
						BlockNumber:     vLog.BlockNumber,
						LogIndex:        vLog.Index,
						Event:           event,
						Data:            vLog,
						ContractAddress: contractAddress,
						BlockHash:       vLog.BlockHash,
					}
					i.eventQueue.Insert(eventLog)
				}
			}(contractConfig, event)
		}
	}
	return nil
}

func (i *Indexer) subscribeToEvent(contractAddress common.Address, event abi.Event) error {
	i.logger.Debug("Subscribing to %s on contract %s", event.Name, contractAddress.String())

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
			i.logger.Info("Received log: %v", vLog)
			eventLog := event_queue.EventLog{
				BlockNumber:     vLog.BlockNumber,
				LogIndex:        vLog.Index,
				Event:           event,
				Data:            vLog,
				ContractAddress: contractAddress,
				BlockHash:       vLog.BlockHash,
			}
			i.eventQueue.Insert(eventLog)
			i.logger.Info("Event inserted into sorted queue: block %d, log index %d", vLog.BlockNumber, vLog.Index)
		case <-i.ctx.Done():
			sub.Unsubscribe()
			return nil
		}
	}
}

func (i *Indexer) loadAbi(contract config.ContractConfig) (abi.ABI, error) {
	abiPath := filepath.Join("abis", contract.ABIPath)
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to read ABI file: %w", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to parse ABI: %w", err)
	}
	return parsedABI, nil
}

func (i *Indexer) Stop() error {
	i.once.Do(func() {
		i.logger.Info("Stopping indexer...")
		i.healthServer.SetStatus("unhealthy")

		// Stop health check server
		if err := i.healthServer.Stop(); err != nil {
			i.logger.Error("Error stopping health check server: %v", err)
		}

		// Signal all goroutines to stop
		close(i.stopChan)

		// Cancel context to stop all child processes
		i.cancel()

		// Stop child processes
		i.extractor.Stop()
		i.transformer.Stop()
		i.finalityProcessor.Stop()

		// Wait for all process monitoring goroutines to finish
		i.processWg.Wait()

		// Close error channel
		close(i.errorChan)

		// Finally release external resources
		i.ethClient.Close()
		i.postgresClient.Close()

		// Tell the process manager the process exited in error
		os.Exit(1)
	})
	return nil
}

func (i *Indexer) getLastIndexedBlock(contractAddress common.Address, event abi.Event) (*int64, error) {
	data, _, err := i.supabaseClient.From("events").
		Select("block_number", "", false).
		Eq("contract_address", contractAddress.String()).
		Eq("event_name", event.Name).
		Order("block_number", &postgrest.OrderOpts{Ascending: false}).
		Limit(1, "").
		Execute()

	if err != nil {
		return nil, err
	}

	var indexedEvents []database.PublicEventsSelect

	if err := json.Unmarshal(data, &indexedEvents); err != nil {
		return nil, err
	}

	if len(indexedEvents) == 0 {
		return nil, nil
	}

	lastIndexedEvent := indexedEvents[0]

	return &lastIndexedEvent.BlockNumber, nil
}

func (i *Indexer) startErrorHandler() {
	go func() {
		for {
			select {
			case err := <-i.errorChan:
				if reorg.IsReorgError(err) {
					i.logger.Error("Reorg detected, initiating cleanup: %v", err)
					i.handleReorg(err.(*reorg.ReorgError))
				} else {
					i.logger.Error("Error: %v", err)
					i.Stop()
				}
				return
			case <-i.ctx.Done():
				// context cancelled, stop listening for errors
				return
			}
		}
	}()
}

// handleReorg handles blockchain reorganization by stopping ingestion, running cleanup, then stopping
func (i *Indexer) handleReorg(reorgErr *reorg.ReorgError) {
	i.logger.Info("Handling reorg at block %d (hash: %s)", reorgErr.BlockNumber, reorgErr.BlockHash)

	// Step 1: Stop event ingestion and processing (but not finality processor yet)
	i.logger.Info("Stopping event ingestion and processing...")
	if i.eventProcessor != nil {
		i.eventProcessor.Stop()
	}

	// Step 2: Stop transformer
	i.logger.Info("Stopping transformer...")
	i.transformer.Stop()

	// Step 3: Run reorg cleanup logic using the dedicated handler
	i.logger.Info("Running reorg cleanup logic...")
	if err := i.reorgHandler.HandleReorg(reorgErr); err != nil {
		i.logger.Error("Error during reorg cleanup: %v", err)
	}

	// Step 4: Stop everything else
	i.logger.Info("Reorg cleanup complete, stopping indexer...")
	i.Stop()
}
