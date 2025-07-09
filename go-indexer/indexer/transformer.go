package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/lib/pq" // Import PostgreSQL driver
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/dbutil"
	transformHandler "github.com/timewave/vault-indexer/go-indexer/event-transform-handler"
	"github.com/timewave/vault-indexer/go-indexer/logger"
	transformers "github.com/timewave/vault-indexer/go-indexer/transformers"
)

// Transformer handles processing of raw events into derived data
type Transformer struct {
	db                         *supa.Client
	pgdb                       *sql.DB
	ctx                        context.Context
	cancel                     context.CancelFunc
	wg                         sync.WaitGroup
	logger                     *logger.Logger
	transformHandler           *transformHandler.TransformHandler
	positionTransformer        *transformers.PositionTransformer
	withdrawRequestTransformer *transformers.WithdrawRequestTransformer
	rateUpdateTransformer      *transformers.RateUpdateTransformer

	// Add retry tracking
	retryCount    int
	maxRetries    int
	lastError     error
	lastErrorTime *time.Time
	once          sync.Once

	// Pause/resume support
	haltCh   chan struct{}
	isHalted bool
	haltMu   sync.Mutex
}

// NewTransformer creates a new transformer instance
func NewTransformer(supa *supa.Client, pgdb *sql.DB, ethClient *ethclient.Client) (*Transformer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure connection pool
	pgdb.SetMaxOpenConns(1) // positions must be processed sequentially, parallel processing is not supported here
	pgdb.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := pgdb.Ping(); err != nil {
		cancel()
		return nil, err
	}

	// initialize transformers
	positionTransformer := transformers.NewPositionTransformer(supa)
	withdrawRequestTransformer := transformers.NewWithdrawRequestTransformer(supa)
	rateUpdateTransformer := transformers.NewRateUpdateTransformer(supa, ethClient)

	// initialize event transformer handlers
	depositHandler := transformHandler.NewDepositHandler(positionTransformer)
	transferHandler := transformHandler.NewTransferHandler(positionTransformer)
	withdrawHandler := transformHandler.NewWithdrawHandler(positionTransformer, withdrawRequestTransformer)
	rateUpdateHandler := transformHandler.NewRateUpdateHandler(rateUpdateTransformer)

	// initialize transform handler
	transformHandler := transformHandler.NewHandler(depositHandler, transferHandler, withdrawHandler, rateUpdateHandler)

	return &Transformer{
		db:                         supa,
		pgdb:                       pgdb,
		ctx:                        ctx,
		cancel:                     cancel,
		logger:                     logger.NewLogger("Transformer"),
		maxRetries:                 5,
		lastError:                  nil,
		retryCount:                 0,
		transformHandler:           transformHandler,
		positionTransformer:        positionTransformer,
		withdrawRequestTransformer: withdrawRequestTransformer,
		rateUpdateTransformer:      rateUpdateTransformer,
		haltCh:                     make(chan struct{}, 1),
		isHalted:                   false,
	}, nil
}

// Start begins the transformation process
func (t *Transformer) Start() error {
	t.logger.Info("Transformer started")

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer close(t.haltCh) // Signal that we've exited
		for {

			select {
			case <-t.haltCh:
				t.logger.Info("Transformer is halted")
				return
			case <-t.ctx.Done():
				return
			default:
				// Query for unprocessed events outside of transaction
				rows, err := t.pgdb.QueryContext(t.ctx, `
					SELECT 
					id,	block_number, contract_address,event_name, log_index, transaction_hash, raw_data, created_at,
					last_updated_at, is_processed
					 FROM events 
					WHERE is_processed is false 
					ORDER BY block_number ASC, log_index ASC 
					LIMIT 100
				`)
				if err != nil {
					t.handleError(err)
					continue
				}

				var events []database.PublicEventsSelect
				for rows.Next() {
					var event database.PublicEventsSelect
					if err := rows.Scan(
						&event.Id,
						&event.BlockNumber,
						&event.ContractAddress,
						&event.EventName,
						&event.LogIndex,
						&event.TransactionHash,
						&event.RawData,
						&event.CreatedAt,
						&event.LastUpdatedAt,
						&event.IsProcessed,
					); err != nil {
						rows.Close()
						t.handleError(err)
						continue
					}
					events = append(events, event)
				}
				rows.Close()

				if len(events) == 0 {
					t.logger.Info("No events to transform, waiting 15 seconds")
					select {
					case <-time.After(15 * time.Second):
					case <-t.ctx.Done():
						return
					}
					continue
				}

				t.logger.Info("Found %d events to transform, processing...", len(events))

				// Process the entire batch, retrying if any event fails
				if err := t.processBatch(events); err != nil {
					t.handleError(err)
					continue
				}

				// Reset retry count after successful batch
				t.retryCount = 0
				t.lastError = nil
			}
		}
	}()
	return nil
}

// processBatch handles the processing of a batch of events, retrying the entire batch if any event fails
func (t *Transformer) processBatch(events []database.PublicEventsSelect) error {
	for _, event := range events {
		t.logger.Debug("Starting DB transaction for %v, id: %v", event.EventName, event.Id)

		tx, err := t.pgdb.BeginTx(t.ctx, &sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
		})
		if err != nil {
			t.logger.Error("Error starting transaction: %v", err)
			return err
		}

		eventUpdate := database.PublicEventsUpdate{
			Id:          &event.Id,
			IsProcessed: ptr(true),
		}

		dbOperations, err := t.transform(event)
		if err != nil {
			t.logger.Error("Error computing transformation: %v", err)
			tx.Rollback()
			return err
		}

		for _, op := range dbOperations.inserts {
			if err := t.insertToTable(tx, op.table, op.data); err != nil {
				t.logger.Error("Error inserting into table %v: %v, data: %v", op.table, err, op.data)
				tx.Rollback()
				return err
			}
		}

		for _, op := range dbOperations.updates {
			if err := t.updateInTable(tx, op.table, op.data); err != nil {
				t.logger.Error("Error updating table %v: %v, data: %v", op.table, err, op.data)
				tx.Rollback()
				return err
			}
		}

		if err := t.updateInTable(tx, "events", eventUpdate); err != nil {
			t.logger.Error("Error updating event: %v", err)
			tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			t.logger.Error("Error committing transaction: %v", err)
			return err
		}

		t.logger.Info("Transformed event: %v", event.Id)
	}

	return nil // Success
}

func (t *Transformer) transform(event database.PublicEventsSelect) (DatabaseOperations, error) {
	operations, err := t.transformHandler.Handle(event)
	if err != nil {
		return DatabaseOperations{}, err
	}

	// Convert handler.DatabaseOperations to transformer.DatabaseOperations
	var dbOps DatabaseOperations
	for _, insert := range operations.Inserts {
		dbOps.inserts = append(dbOps.inserts, DBOperation{
			table: insert.Table,
			data:  insert.Data,
		})
	}
	for _, update := range operations.Updates {
		dbOps.updates = append(dbOps.updates, DBOperation{
			table: update.Table,
			data:  update.Data,
		})
	}

	return dbOps, nil
}

// handleError manages retry logic and backoff
func (t *Transformer) handleError(err error) {

	now := time.Now()
	t.logger.Debug("Error occurred: err %v lastError %v, retryCount: %v", err, t.lastError, t.retryCount)

	// If this is the same error as before
	if t.lastError != nil {
		t.retryCount++
		t.logger.Debug("Incrementing retry count: %v", t.retryCount)
		if t.retryCount >= t.maxRetries {
			t.logger.Error("FAILURE: Max retries (%d) reached for error: %v. Halting the transformer.", t.maxRetries, err)
			t.cancel()
			return
		}
	} else {
		t.lastError = err
		t.retryCount = 0
	}

	// Calculate backoff duration (exponential backoff with jitter)
	backoff := time.Duration(t.retryCount*t.retryCount) * time.Second
	jitter := time.Duration(rand.Int63n(int64(time.Second)))

	backoff += jitter

	t.logger.Warn("Error occurred (retry %d/%d): %v. Backing off for %v",
		t.retryCount, t.maxRetries, err, backoff)

	t.lastErrorTime = &now
	select {
	case <-time.After(backoff):
		t.logger.Debug("Backoff complete, retrying...")
	case <-t.ctx.Done():
		t.logger.Debug("Context cancelled during backoff")
		return
	}
}

// handleCleanupFromBlock handles the cleanup of the database from a given block number
func (t *Transformer) handleCleanupFromBlock(blockNumber int64, pgdb *sql.DB) error {

	if !t.isHalted {
		return fmt.Errorf("transformer is not halted, cannot safely do reorg cleanup")
	}

	t.logger.Info("Cleaning up from block: %v", blockNumber)

	transformersArr := []interface {
		CleanupFromBlock() []string
	}{
		t.withdrawRequestTransformer,
		t.rateUpdateTransformer,
		t.positionTransformer,
	}
	tx, err := pgdb.BeginTx(t.ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	t.logger.Info("Begin reorg cleanup transaction")
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	for _, transformer := range transformersArr {
		sqls := transformer.CleanupFromBlock()
		for _, sql := range sqls {
			_, err := tx.Exec(sql, blockNumber)
			if err != nil {
				tx.Rollback()
				t.logger.Error("failed to handle cleanup for %v: %v", transformer, err)
				return fmt.Errorf("failed to handle cleanup for %v: %v", transformer, err)
			}
			t.logger.Info("Successfully executed cleanup sql: %v", sql)
		}
	}

	_, err = tx.Exec("DELETE FROM events WHERE block_number >= $1", blockNumber)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete events: %v", err)
	}

	t.logger.Info("Committing reorg cleanup transaction")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	t.logger.Info("Successfully cleaned up from block: %v", blockNumber)

	return nil

}

// Halt halts the transformer processing
// specifically for stopping processing to do reorg cleanup
func (t *Transformer) Halt() {
	t.haltMu.Lock()
	defer t.haltMu.Unlock()
	if !t.isHalted {
		t.isHalted = true
		// non-blocking send in case multiple Pause() calls
		select {
		case t.haltCh <- struct{}{}:
		default:
		}
		t.logger.Info("Transformer halt requested")
	}
}
func (t *Transformer) WaitHalted(timeout time.Duration) bool {
	select {
	case <-t.haltCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Stop gracefully stops the transformer
func (t *Transformer) Stop() {
	t.once.Do(func() {
		t.logger.Info("Stopping transformer...")
		t.cancel()
		t.wg.Wait()

		if t.pgdb != nil {
			t.pgdb.Close()
		}
	})
}

func (t *Transformer) insertToTable(tx *sql.Tx, table string, data any) error {
	t.logger.Debug("Building insert for table: %v, data: %v", table, data)
	query, args, err := dbutil.BuildInsert(table, data)
	if err != nil {
		return err
	}
	t.logger.Debug("Executing insert: %v, args: %v", query, args)
	_, err = tx.Exec(query, args...)
	if err != nil {
		t.logger.Error("DB insert error: %v", err)
		return err
	}
	return nil
}

func (t *Transformer) updateInTable(tx *sql.Tx, table string, data any) error {
	t.logger.Debug("Building update for table: %v, data: %v", table, data)
	query, args, err := dbutil.BuildUpdate(table, data, []string{"id"})
	if err != nil {
		return err
	}
	t.logger.Debug("Executing update: %v, args: %v", query, args)
	_, err = tx.Exec(query, args...)
	if err != nil {
		t.logger.Error("DB update error: %v", err)
		return err
	}
	return nil
}

type DBOperation struct {
	table string
	data  any
}

type DatabaseOperations struct {
	inserts []DBOperation
	updates []DBOperation
}

func ptr[T any](v T) *T {
	return &v
}
