package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	_ "github.com/lib/pq" // Import PostgreSQL driver
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/dbutil"
	"github.com/timewave/vault-indexer/go-indexer/logger"
	positionTransformer "github.com/timewave/vault-indexer/go-indexer/transformer"
)

// Transformer handles processing of raw events into derived data
type Transformer struct {
	db                  *supa.Client
	pgdb                *sql.DB
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	logger              *logger.Logger
	positionTransformer *positionTransformer.PositionTransformer

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
func NewTransformer(supa *supa.Client, pgdb *sql.DB) (*Transformer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure connection pool
	pgdb.SetMaxOpenConns(1)
	pgdb.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := pgdb.Ping(); err != nil {
		cancel()
		return nil, err
	}

	return &Transformer{
		db:                  supa,
		pgdb:                pgdb,
		ctx:                 ctx,
		cancel:              cancel,
		logger:              logger.NewLogger("Transformer"),
		maxRetries:          0,
		lastError:           nil,
		retryCount:          0,
		positionTransformer: positionTransformer.NewPositionTransformer(supa),
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
					LIMIT 1
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

					// Start a new transaction for each event
					t.logger.Printf("Starting transaction for event: %v", event.Id)
					tx, err := t.pgdb.BeginTx(t.ctx, &sql.TxOptions{
						Isolation: sql.LevelReadCommitted,
					})
					if err != nil {
						t.logger.Printf("Error starting transaction: %v", err)
						t.handleError(err)
						continue
					}

					eventUpdate := database.PublicEventsUpdate{
						Id:          &event.Id,
						IsProcessed: ptr(true),
					}

					dbOperations, err := t.computeTransformation(event)
					if err != nil {
						t.logger.Printf("Error computing transformation: %v", err)
						tx.Rollback()
						t.handleError(err)
					}

					for _, insertOperation := range dbOperations.inserts {
						t.logger.Printf("Inserting: %v", insertOperation)
						err = t.insertTable(tx, insertOperation.table, insertOperation.data)
						if err != nil {
							t.logger.Printf("Error inserting position: %v", err)
							tx.Rollback()
							t.handleError(err)
							break
						}
					}

					for _, updateOperation := range dbOperations.updates {
						t.logger.Printf("Updating : %v", updateOperation)

						err = t.updateTable(tx, updateOperation.table, updateOperation.data)
						if err != nil {
							t.logger.Printf("Error updating position: %v", err)
							tx.Rollback()
							t.handleError(err)
							break
						}
					}

					err = t.updateTable(tx, "events", eventUpdate)
					if err != nil {
						t.logger.Printf("Error updating event: %v", err)
						tx.Rollback()
						t.handleError(err)
						continue
					}

					t.logger.Printf("Committing transaction for event: %v", event.Id)
					if err := tx.Commit(); err != nil {
						t.logger.Printf("Error committing transaction: %v", err)
						t.handleError(err)
						continue
					}
					t.logger.Printf("Successfully committed transaction for event: %v", event.Id)

				}

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

func (t *Transformer) insertTable(tx *sql.Tx, table string, data any) error {
	t.logger.Printf("building insert for table: %v, data: %v", table, data)
	query, args, err := dbutil.BuildInsert(table, data)
	if err != nil {
		return err
	}
	t.logger.Printf("executing insert: %v", query)
	res, err := tx.Exec(query, args...)
	t.logger.Printf("insert error: %v", err)
	if err != nil {
		return err
	}
	t.logger.Printf("insert result: %v", res)
	return nil
}

func (t *Transformer) updateTable(tx *sql.Tx, table string, data any) error {
	t.logger.Printf("building update")
	query, args, err := dbutil.BuildUpdate(table, data, []string{"id"})
	if err != nil {
		return err
	}
	t.logger.Printf("executing update: %v", query)
	res, err := tx.Exec(query, args...)
	t.logger.Printf("update error: %v", err)
	if err != nil {
		return err
	}
	t.logger.Printf("update result: %v", res)
	return nil
}

func (t *Transformer) computeTransformation(event database.PublicEventsSelect) (DatabaseOperations, error) {

	var operations DatabaseOperations
	var eventData, err = parseQuotedBased64Json(event)
	if err != nil {
		return operations, err
	}
	t.logger.Printf("NEW: %v %v eventData: %v", event.ContractAddress, event.EventName, eventData)

	if event.EventName == "Transfer" {

		var senderAddress string
		var receiverAddress string

		if to, ok := eventData["to"].(string); ok {
			receiverAddress = to
		} else {
			return operations, errors.New("to not found")
		}
		if from, ok := eventData["from"].(string); ok {
			senderAddress = from
		} else {
			return operations, errors.New("from not found")
		}

		var amountShares = fmt.Sprintf("%.0f", eventData["value"])

		inserts, updates, err := t.positionTransformer.ProcessPositionTransformation(positionTransformer.ProcessPosition{
			ReceiverAddress: receiverAddress,
			SenderAddress:   senderAddress,
			ContractAddress: event.ContractAddress,
			AmountShares:    amountShares,
			BlockNumber:     uint64(event.BlockNumber),
		}, false)
		t.logger.Printf("inserts: %v", inserts)
		t.logger.Printf("updates: %v", updates)

		if err != nil {
			return operations, err
		}

		if len(inserts) > 0 {

			operations.inserts = append(operations.inserts, DBOperation{
				table: "positions",
				data:  inserts,
			})
		}
		if len(updates) > 0 {

			operations.updates = append(operations.updates, DBOperation{
				table: "positions",
				data:  updates,
			})
		}

		return operations, nil

	} else if event.EventName == "WithdrawRequested" {

		var senderAddress string
		var receiverAddress string

		if from, ok := eventData["owner"].(string); ok {
			senderAddress = from
		}
		if to, ok := eventData["receiver"].(string); ok {
			receiverAddress = to
		}

		// Handle value that could be either string or numeric
		var amountShares = fmt.Sprintf("%.0f", eventData["shares"])

		inserts, updates, err := t.positionTransformer.ProcessPositionTransformation(positionTransformer.ProcessPosition{
			ReceiverAddress: receiverAddress,
			SenderAddress:   senderAddress,
			ContractAddress: event.ContractAddress,
			AmountShares:    amountShares,
			BlockNumber:     uint64(event.BlockNumber),
		}, true)
		if err != nil {
			return operations, err
		}
		t.logger.Printf("inserts: %v", inserts)
		t.logger.Printf("updates: %v", updates)

		if len(inserts) > 0 {
			operations.inserts = append(operations.inserts, DBOperation{
				table: "positions",
				data:  inserts,
			})
		}

		if len(updates) > 0 {
			operations.updates = append(operations.updates, DBOperation{
				table: "positions",
				data:  updates,
			})
		}

		withdrawRequest := database.PublicWithdrawRequestsInsert{
			Amount:          amountShares,
			BlockNumber:     int64(event.BlockNumber),
			ContractAddress: event.ContractAddress,
			OwnerAddress:    senderAddress,
			ReceiverAddress: receiverAddress,
			WithdrawId:      int64(event.BlockNumber),
		}
		operations.inserts = append(operations.inserts, DBOperation{
			table: "withdraw_requests",
			data:  withdrawRequest,
		})

		return operations, nil
	}

	return operations, nil
}

type DBOperation struct {
	table string
	data  any
}

type DatabaseOperations struct {
	inserts []DBOperation
	updates []DBOperation
}
