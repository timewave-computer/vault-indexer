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
	once                  sync.Once
	errorChan             chan error
	reorgChan             chan ReorgEvent
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

	finalityProcessor := NewFinalityProcessor(ethClient, supabaseClient)

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
	healthServer := health.NewServer(8080, "indexer") // Default port for indexer health checks

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
		eventQueue:            eventQueue,
		requiredConfirmations: 4,
		errorChan:             make(chan error),
		reorgChan:             make(chan ReorgEvent),
		processWg:             sync.WaitGroup{},
		eventProcessor:        eventProcessor,
	}, nil
}

func (i *Indexer) Start() error {
	i.logger.Info("Indexer started")

	// Start centralized error and reorg event handler
	i.startErrorAndReorgMonitoring()

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
			select {
			case i.errorChan <- fmt.Errorf("event processor stopped"):
			default:
				// Channel is closed or full, ignore
			}

		}
	}()

	i.processWg.Add(1)
	go func() {
		defer i.processWg.Done()
		select {
		case <-i.transformer.ctx.Done():
			i.logger.Warn("Transformer context cancelled, stopping indexer")
			select {
			case i.errorChan <- fmt.Errorf("transformer stopped"):
			default:
				// Channel is closed or full, ignore
			}
		default:
		}
	}()

	i.processWg.Add(1)
	go func() {
		defer i.processWg.Done()
		select {
		case <-i.finalityProcessor.ctx.Done():
			i.logger.Warn("Finality processor context cancelled, stopping indexer")
			select {
			case i.errorChan <- fmt.Errorf("finality processor stopped"):
			default:
				// Channel is closed or full, ignore
			}
		default:
		}
	}()

	// Monitor reorg events from finality processor
	i.processWg.Add(1)
	go func() {
		defer i.processWg.Done()
		for {
			select {
			case reorgEvent := <-i.finalityProcessor.ReorgChannel():
				i.logger.Info("Reorg event received from finality processor: %+v", reorgEvent)
				select {
				case i.reorgChan <- reorgEvent:
					i.logger.Info("Reorg event forwarded to main event handler")
				default:
					i.logger.Warn("Reorg channel is full, dropping event")
				}
			case <-i.ctx.Done():
				return
			}
		}
	}()

	// Wait for context cancellation or stop signal
	select {
	case <-i.ctx.Done():
		i.logger.Info("Context cancelled, shutting down indexer...")
		i.Stop()
	}

	return nil
}

func (i *Indexer) Stop() error {
	i.once.Do(func() {
		i.logger.Info("Stopping indexer...")
		i.healthServer.SetStatus("unhealthy")

		// Stop health check server
		if err := i.healthServer.Stop(); err != nil {
			i.logger.Error("Error stopping health check server: %v", err)
		}

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

		i.logger.Info("Indexer stopped successfully")
		os.Exit(1)
	})
	return nil
}

func (i *Indexer) startErrorAndReorgMonitoring() {
	go func() {
		for {
			select {
			case err := <-i.errorChan:
				i.logger.Error("Error received: %v", err)
				i.Stop()
				return
			case reorgEvent := <-i.reorgChan:
				i.logger.Error("Reorg event received: %+v", reorgEvent)
				i.handleReorg(reorgEvent)
				return
			case <-i.ctx.Done():
				// context cancelled, stop listening for events
				return
			}
		}
	}()
}
