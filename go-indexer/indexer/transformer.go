package indexer

import (
	"context"
	"database/sql"
	"math/rand"
	"sync"
	"time"

	"errors"

	_ "github.com/lib/pq" // Import PostgreSQL driver
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/dbutil"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// Transformer handles processing of raw events into derived data
type Transformer struct {
	db     *supa.Client
	pgdb   *sql.DB
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *logger.Logger
	// Add retry tracking
	retryCount    int
	maxRetries    int
	lastError     error
	lastErrorTime *time.Time
}

type manualError string

func (e manualError) Error() string {
	return string(e)
}

func (e manualError) Is(target error) bool {
	if t, ok := target.(manualError); ok {
		return e == t
	}
	return false
}

// NewTransformer creates a new transformer instance
func NewTransformer(db *supa.Client) (*Transformer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pgConnStr := "postgresql://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable"
	// Open PostgreSQL connection
	pgdb, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		cancel()
		return nil, err
	}

	// Configure connection pool
	pgdb.SetMaxOpenConns(1)
	pgdb.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := pgdb.Ping(); err != nil {
		cancel()
		return nil, err
	}

	return &Transformer{
		db:         db,
		pgdb:       pgdb,
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger.NewLogger("Transformer"),
		maxRetries: 1,
		lastError:  nil,
		retryCount: 0,
	}, nil
}

// Start begins the transformation process
func (t *Transformer) Start() error {
	t.logger.Println("Starting transformer...")
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
				// Query for unprocessed events outside of transaction
				rows, err := t.pgdb.QueryContext(t.ctx, `
					SELECT 
					id,	block_number, contract_address,event_name, log_index, transaction_hash, raw_data, created_at,
					last_updated_at, is_processed
					 FROM events 
					WHERE is_processed IS NULL 
					ORDER BY block_number ASC, log_index ASC 
					LIMIT 2
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
					t.logger.Printf("No events found, waiting 15 seconds")
					select {
					case <-time.After(15 * time.Second):
					case <-t.ctx.Done():
						return
					}
					continue
				}

				t.logger.Printf("Received %d events, processing...", len(events))

				// Process each event in its own transaction
				for _, event := range events {
					t.logger.Printf("Processing event: %v", event)

					// Start a new transaction for each event
					tx, err := t.pgdb.BeginTx(t.ctx, &sql.TxOptions{
						Isolation: sql.LevelReadCommitted,
					})
					if err != nil {
						t.handleError(err)
						continue
					}

					eventUpdate := database.PublicEventsUpdate{
						Id:          &event.Id,
						IsProcessed: ptr(true),
					}

					query, args, err := dbutil.BuildUpdate("events", eventUpdate, []string{"id"})
					if err != nil {
						tx.Rollback()
						t.handleError(err)
						continue
					}

					_, err = tx.ExecContext(t.ctx, query, args...)
					if err != nil {
						tx.Rollback()
						t.handleError(err)
						continue
					}

					// Commit the transaction
					if err := tx.Commit(); err != nil {
						t.handleError(err)
						continue
					}

				}

				t.handleError(manualError("end after first few events"))
				// TODO: REMOVE!!
				continue

				// Reset retry count on successful operation
				t.retryCount = 0
				t.lastError = nil
			}
		}
	}()
	return nil
}

// handleError manages retry logic and backoff
func (t *Transformer) handleError(err error) {
	now := time.Now()
	t.logger.Printf("Error occurred: err %v lastError %v", err, t.lastError)

	// If this is the same error as before
	if t.lastError != nil && errors.Is(err, t.lastError) {
		t.retryCount++
		if t.retryCount >= t.maxRetries {
			t.logger.Printf("FAILURE: Max retries (%d) reached for error: %v. Stopping transformer.", t.maxRetries, err)
			t.cancel()
			return
		}
	} else {
		t.lastError = err
		t.retryCount = 1
	}

	// Calculate backoff duration (exponential backoff with jitter)
	backoff := time.Duration(t.retryCount*t.retryCount) * time.Second
	jitter := time.Duration(rand.Int63n(int64(time.Second)))
	backoff += jitter

	t.logger.Printf("Error occurred (retry %d/%d): %v. Backing off for %v",
		t.retryCount, t.maxRetries, err, backoff)

	t.lastErrorTime = &now
	select {
	case <-time.After(backoff):
	case <-t.ctx.Done():
		return
	}
}

// Stop gracefully stops the transformer
func (t *Transformer) Stop() {
	t.cancel()
	t.wg.Wait()
	if t.pgdb != nil {
		t.pgdb.Close()
	}
}

func UpdateEvent(db *sql.DB, insert database.PublicEventsUpdate) error {
	query, args, err := dbutil.BuildInsert("events", insert)
	if err != nil {
		return err
	}
	_, err = db.Exec(query, args...)
	return err
}

func ptr[T any](v T) *T {
	return &v
}
