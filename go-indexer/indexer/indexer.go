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
	extractor      *Extractor
	transformer    *Transformer
	subscriptionWG sync.WaitGroup
	logger         *logger.Logger
	healthServer   *health.Server
	errors         chan error
}

func New(ctx context.Context, cfg *config.Config) (*Indexer, error) {
	ctx, cancel := context.WithCancel(ctx)
	indexerLogger := logger.NewLogger("Indexer")

	indexerLogger.Info("Starting indexer...")

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

	errors := make(chan error, len(cfg.Contracts)*10)

	return &Indexer{
		config:         cfg,
		postgresClient: postgresClient,
		ethClient:      ethClient,
		supabaseClient: supabaseClient,
		ctx:            ctx,
		cancel:         cancel,
		extractor:      extractor,
		transformer:    transformer,
		logger:         indexerLogger,
		healthServer:   healthServer,
		errors:         errors,
	}, nil
}

func (i *Indexer) Start() error {
	i.logger.Info("Starting indexer...")

	if err := i.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	currentBlock, err := i.ethClient.BlockNumber(i.ctx)
	if err != nil {
		i.Stop()
		return fmt.Errorf("failed to get current block height: %w", err)
	}
	i.logger.Info("Current block height: %d", currentBlock)

	// Listen for errors from go routines
	go func() {
		select {
		case err := <-i.errors:
			i.logger.Error("Error: %v", err)
			i.Stop()
		case <-i.ctx.Done():
			// context cancelled, stop listening for errors
			return
		}
	}()

	/*
		1. Start subscription and writing events immediately
	*/
	i.logger.Info("Setting up subscriptions for %d contracts...", len(i.config.Contracts))
	i.setupSubscriptions()
	i.subscriptionWG.Wait()

	i.logger.Info("Subscriptions created")

	/*
		2. Backfill with idempotency [lastProcessedBlock,currentBlock]
	*/

	// i.logger.Info("Backfilling events from current block %d...", currentBlock)
	// i.backfillEvents(currentBlock)
	// i.logger.Info("Backfill completed")

	/*
		3. Start transformer (process events saved in database)
	*/

	if err := i.transformer.Start(); err != nil {
		return fmt.Errorf("failed to start transformer: %w", err)
	}

	go func() {
		<-i.transformer.ctx.Done()
		i.transformer.Stop()
	}()

	// Wait for context cancellation
	<-i.ctx.Done()
	i.logger.Info("Context cancelled, shutting down indexer...")

	return nil
}

func (i *Indexer) setupSubscriptions() {
	for _, contract := range i.config.Contracts {
		parsedABI, err := i.loadAbi(contract)
		if err != nil {
			i.sendError(fmt.Errorf("failed to load ABI for contract %s: %w", contract.Address, err))
			return
		}
		contractAddress := common.HexToAddress(contract.Address)

		for _, _eventName := range contract.Events {
			eventName := parsedABI.Events[_eventName]

			i.subscriptionWG.Add(1)
			go func(event abi.Event) {
				// Signal that this subscription goroutine has started
				i.subscriptionWG.Done()

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
						// Use non-blocking send to avoid panic if channel is closed
						select {
						case i.errors <- fmt.Errorf("failed to subscribe to event %s for contract %s: %w", event.Name, contract.Address, err):
						default:
							// Channel is closed or full, ignore
						}
						i.Stop()
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
}

func (i *Indexer) subscribeToEvent(contractAddress common.Address, event abi.Event) error {
	i.logger.Debug("Subscribing to %s on contract %s", event.Name, contractAddress.String())

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{event.ID}},
		FromBlock: big.NewInt(int64(22694852)),
		ToBlock:   big.NewInt(int64(22783028)),
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
			if err := i.extractor.writeIdempotentEvent(vLog, event); err != nil {
				i.logger.Error("Error processing event: %v", err)
				return err
			}
		case <-i.ctx.Done():
			sub.Unsubscribe()
			return nil
		}
	}
}

func (i *Indexer) backfillEvents(currentBlock uint64) {
	for _, contract := range i.config.Contracts {

		parsedABI, err := i.loadAbi(contract)
		if err != nil {
			i.sendError(fmt.Errorf("failed to load ABI for contract %s: %w", contract.Address, err))
			return
		}

		contractAddress := common.HexToAddress(contract.Address)

		// Create filter query for each event
		for _, eventName := range contract.Events {

			lastIndexedBlock, err := i.getLastIndexedBlock(contractAddress, eventName)
			if err != nil {
				i.sendError(fmt.Errorf("failed to get last processed event for contract %s and event %s: %w", contract.Address, eventName, err))
				return
			}

			toBlock := big.NewInt(int64(currentBlock))

			var fromBlock *big.Int
			if lastIndexedBlock == nil {
				i.logger.Info("Backfilling from start block %d for %s on %s", contract.StartBlock, eventName, contract.Address)
				fromBlock = big.NewInt(int64(contract.StartBlock))
			} else {
				i.logger.Info("Backfilling from last indexed block %d for %s on %s", *lastIndexedBlock, eventName, contract.Address)
				fromBlock = big.NewInt(int64(*lastIndexedBlock)) // its ok if there is overlap in the last indexed block, events are idempotent
			}

			// Check if context is cancelled before processing each event
			select {
			case <-i.ctx.Done():
				// Don't send error on context cancellation during shutdown
				return
			default:
			}

			event, exists := parsedABI.Events[eventName]
			if !exists {
				i.sendError(fmt.Errorf("event %s not found in ABI", eventName))
				return
			}

			query := ethereum.FilterQuery{
				FromBlock: fromBlock,
				ToBlock:   toBlock,
				Addresses: []common.Address{contractAddress},
				Topics:    [][]common.Hash{{event.ID}},
			}

			logs, err := i.ethClient.FilterLogs(i.ctx, query)
			if err != nil {
				i.sendError(fmt.Errorf("failed to get logs: %w", err))
				return
			}

			// Process each log
			for _, vLog := range logs {

				// Check if context is cancelled before processing each log
				select {
				case <-i.ctx.Done():
					// Don't send error on context cancellation during shutdown
					return
				default:
				}

				if err := i.extractor.writeIdempotentEvent(vLog, event); err != nil {
					i.sendError(fmt.Errorf("failed to process event: %w", err))
					return
				}
				i.logger.Info("Sleeping for 15 seconds during backfill")
				time.Sleep(15 * time.Second)
			}
		}
	}
}

func (i *Indexer) getLastIndexedBlock(
	contractAddress common.Address,
	eventName string,
) (*int64, error) {

	data, _, err := i.supabaseClient.From("events").
		Select("block_number", "", false).
		Eq("contract_address", contractAddress.String()).
		Eq("event_name", eventName).
		Eq("is_pending_backfill", "false").
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

	i.logger.Debug("Last indexed block for %s on %s: %v", eventName, contractAddress.String(), indexedEvents)

	if len(indexedEvents) == 0 {
		return nil, nil
	}

	lastIndexedEvent := indexedEvents[0]

	return &lastIndexedEvent.BlockNumber, nil
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

// sendError safely sends an error to the errors channel without panicking
func (i *Indexer) sendError(err error) {
	select {
	case i.errors <- err:
	default:
		// Channel is closed or full, ignore
		i.logger.Error("Error sending error to channel: %v", err)
	}
}

func (i *Indexer) Stop() error {
	i.logger.Info("Stopping indexer...")
	i.healthServer.SetStatus("unhealthy")

	// Stop health check server
	if err := i.healthServer.Stop(); err != nil {
		i.logger.Error("Error stopping health check server: %v", err)
	}

	// signal writers to stop
	i.cancel()

	// wait until all goroutines finish
	i.subscriptionWG.Wait()

	i.extractor.Stop()
	i.transformer.Stop()

	// finally release external resources
	i.ethClient.Close()
	i.postgresClient.Close()

	return nil
}
