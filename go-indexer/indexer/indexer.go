package indexer

import (
	"context"
	"database/sql"

	"fmt"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/config"
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
	stopChan              chan StopChannel
	processWg             sync.WaitGroup
	eventProcessor        *EventProcessor
	reorgCh               chan struct{} // Channel to signal reorg to workers
}

func New(ctx context.Context, cfg *config.Config, logger *logger.Logger) (*Indexer, error) {
	ctx, cancel := context.WithCancel(ctx)

	healthServer := health.NewServer(8080, "indexer") // Default port for indexer health checks

	postgresClient, err := sql.Open("postgres", cfg.Database.PostgresConnectionString)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Postgres: %w", err)
	}
	logger.Info("Connected to pgdb")

	ethClient, err := ethclient.Dial(cfg.Ethereum.WebsocketURL)
	logger.Info("Dialed to Ethereum websocket")
	if err != nil {
		cancel()
		return nil, err
	}
	logger.Info("Connected to Ethereum websocket")

	supabaseClient, err := supa.NewClient(cfg.Database.SupabaseURL, cfg.Database.SupabaseKey, &supa.ClientOptions{})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Supabase: %w", err)
	}
	logger.Info("Connected to supabase client")

	extractor := NewExtractor(supabaseClient, ethClient)
	stopChan := NewStopChannel()

	eventQueue := event_queue.NewEventQueue()

	eventProcessor := NewEventProcessor(eventQueue, extractor, ethClient, 4, ctx, stopChan)

	finalityProcessor := NewFinalityProcessor(ethClient, supabaseClient, ctx, stopChan)

	transformer, err := NewTransformer(supabaseClient, postgresClient, ethClient, ctx, stopChan)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create transformer: %w", err)
	}

	// Create reorg handler
	reorgHandler := reorg.NewHandler(supabaseClient, postgresClient, ethClient, cfg)

	return &Indexer{
		config:                cfg,
		postgresClient:        postgresClient,
		ethClient:             ethClient,
		supabaseClient:        supabaseClient,
		ctx:                   ctx,
		cancel:                cancel,
		extractor:             extractor,
		transformer:           transformer,
		logger:                logger,
		healthServer:          healthServer,
		finalityProcessor:     finalityProcessor,
		reorgHandler:          reorgHandler,
		eventQueue:            eventQueue,
		requiredConfirmations: 4,
		processWg:             sync.WaitGroup{},
		eventProcessor:        eventProcessor,
		reorgCh:               make(chan struct{}),
	}, nil
}

func (i *Indexer) Start() error {
	i.logger.Info("Indexer started")

	// Start centralized error handler
	i.startErrorHandler()

	if err := i.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	currentBlock, err := i.ethClient.BlockNumber(i.ctx)
	if err != nil {
		i.Stop()
		return fmt.Errorf("failed to get current block height: %w", err)
	}
	i.logger.Info("Current block height: %d", currentBlock)
	i.logger.Debug("Creating subscriptions for %d contracts...", len(i.config.Contracts))

	wg := sync.WaitGroup{}
	err = i.setupSubscriptions(&wg)
	wg.Wait() // do not progress until all subscriptions are created
	if err != nil {
		return fmt.Errorf("failed to set up subscriptions: %w", err)
	}
	i.logger.Info("Subscriptions created")

	i.finalityProcessor.Start()

	err = i.loadHistoricalEvents(currentBlock, &wg)
	wg.Wait() // do not progress until all historical events are loaded
	if err != nil {
		return fmt.Errorf("failed to backfill events: %w", err)
	}

	i.logger.Info("Loaded historical events into ingestion queue. Size: %d", i.eventQueue.Len())

	err = i.eventProcessor.Start()
	if err != nil {
		return fmt.Errorf("event queue processor failure: %w", err)
	}

	if err := i.transformer.Start(); err != nil {
		return fmt.Errorf("failed to start transformer: %w", err)
	}

	// Wait for context cancellation or stop signal
	select {
	case <-i.ctx.Done():
		i.logger.Info("Context cancelled, shutting down indexer...")
	case <-i.reorgCh:
		i.logger.Info("Reorg signaled, shutting down indexer...")
	}

	return nil
}

func (i *Indexer) startErrorHandler() {
	go func() {
		for {
			select {
			case stopSignal := <-i.stopChan:
				if stopSignal.isReorg {
					i.logger.Error("Reorg detected, initiating cleanup: %v", stopSignal.blockNumber, stopSignal.blockHash)
					i.handleReorg(stopSignal.blockNumber, stopSignal.blockHash)
				} else {
					i.logger.Error("Error: %v", stopSignal.blockNumber, stopSignal.blockHash)
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
func (i *Indexer) handleReorg(blockNumber int64, blockHash string) {
	i.once.Do(func() {
		i.logger.Info("Handling reorg at block %d (hash: %s)", blockNumber, blockHash)

		// Step 3: Run reorg cleanup logic using the dedicated handler
		i.logger.Info("Running reorg cleanup logic...")
		if err := i.reorgHandler.HandleReorg(blockNumber, blockHash); err != nil {
			i.logger.Error("Error during reorg cleanup: %v", err)
		}

		// Step 4: Stop everything else
		i.logger.Info("Reorg cleanup complete, stopping indexer...")
		i.Stop()
	})
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
		i.safeCloseChannel(i.reorgCh)

		// Cancel context to stop all child processes
		i.cancel()

		// Wait for all process monitoring goroutines to finish
		i.processWg.Wait()

		// Close error channel
		close(i.stopChan)

		// Finally release external resources
		i.ethClient.Close()
		i.postgresClient.Close()

		// Tell the process manager the process exited in error
		os.Exit(1)
	})
	return nil
}

func (i *Indexer) safeCloseChannel(channel chan struct{}) {
	i.once.Do(func() {
		close(channel)
	})
}
