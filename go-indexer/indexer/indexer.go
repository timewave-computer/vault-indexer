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
	extractor      *Extractor
	transformer    *Transformer
	wg             sync.WaitGroup
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
	i.logger.Info("Creating subscriptions for %d contracts...", len(i.config.Contracts))
	i.setupSubscriptions()

	/*
		2. Backfill with idempotency [lastProcessedBlock,currentBlock]
	*/

	i.backfillEvents(currentBlock)

	/*
		3. Start transformer
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

func (i *Indexer) backfillEvents(currentBlock uint64) {
	for _, contract := range i.config.Contracts {
		parsedABI, err := i.loadAbi(contract)
		if err != nil {
			i.errors <- fmt.Errorf("failed to load ABI for contract %s: %w", contract.Address, err)
			return
		}

		contractAddress := common.HexToAddress(contract.Address)

		// Create filter query for each event
		for _, eventName := range contract.Events {
			// Check if context is cancelled before processing each event
			select {
			case <-i.ctx.Done():
				i.errors <- fmt.Errorf("context cancelled during historical event processing")
				return
			default:
			}

			event, exists := parsedABI.Events[eventName]
			if !exists {
				i.errors <- fmt.Errorf("event %s not found in ABI", eventName)
			}

			var toBlock *big.Int
			if contract.EndBlock == 0 {
				toBlock = big.NewInt(int64(currentBlock)) // add generous buffer for backfilling
			} else {
				toBlock = big.NewInt(int64(contract.EndBlock))
			}
			fromBlock := big.NewInt(int64(contract.StartBlock))

			query := ethereum.FilterQuery{
				FromBlock: fromBlock,
				ToBlock:   toBlock,
				Addresses: []common.Address{contractAddress},
				Topics:    [][]common.Hash{{event.ID}},
			}

			logs, err := i.ethClient.FilterLogs(i.ctx, query)
			if err != nil {
				i.errors <- fmt.Errorf("failed to get logs: %w", err)
				return
			}

			// Process each log
			for _, vLog := range logs {
				// Check if context is cancelled before processing each log
				select {
				case <-i.ctx.Done():
					i.errors <- fmt.Errorf("context cancelled during log processing")
					return
				default:
				}

				if err := i.extractor.writeEvent(vLog, event); err != nil {
					i.errors <- fmt.Errorf("failed to process event: %w", err)
				}
			}
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

func (i *Indexer) setupSubscriptions() {
	for _, contract := range i.config.Contracts {
		parsedABI, err := i.loadAbi(contract)
		if err != nil {
			i.errors <- fmt.Errorf("failed to load ABI for contract %s: %w", contract.Address, err)
			return
		}
		contractAddress := common.HexToAddress(contract.Address)

		for _, _eventName := range contract.Events {
			eventName := parsedABI.Events[_eventName]

			i.wg.Add(1)
			go func(event abi.Event) {
				defer i.wg.Done()
				for {
					// Check if context is cancelled before attempting subscription
					select {
					case <-i.ctx.Done():
						i.errors <- fmt.Errorf("context cancelled during event subscription")
						return
					default:
						// Continue with subscription
					}

					err := i.subscribeToEvent(contractAddress, event)

					if err != nil {
						i.errors <- fmt.Errorf("failed to subscribe to event %s for contract %s: %w", event.Name, contract.Address, err)
						i.Stop()
						return
					}

					// Check context again before sleeping
					select {
					case <-i.ctx.Done():
						i.errors <- fmt.Errorf("context cancelled during event subscription")
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
			if err := i.extractor.writeEvent(vLog, event); err != nil {
				i.logger.Error("Error processing event: %v", err)
			}
		case <-i.ctx.Done():
			sub.Unsubscribe()
			return nil
		}
	}
}

func (i *Indexer) Stop() error {
	i.logger.Info("Stopping indexer...")
	i.healthServer.SetStatus("unhealthy")

	close(i.errors)
	// Stop health check server
	if err := i.healthServer.Stop(); err != nil {
		i.logger.Error("Error stopping health check server: %v", err)
	}

	// signal writers to stop
	i.cancel()

	// wait until all goroutines finish
	i.wg.Wait()

	i.extractor.Stop()
	i.transformer.Stop()

	// finally release external resources
	i.ethClient.Close()
	i.postgresClient.Close()

	return nil
}
