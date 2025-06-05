package indexer

import (
	"context"
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"errors"

	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

// Transformer handles processing of raw events into derived data
type Transformer struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *Logger
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
func NewTransformer(db *supa.Client) *Transformer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Transformer{
		db:         db,
		ctx:        ctx,
		cancel:     cancel,
		logger:     NewLogger("Transformer"),
		maxRetries: 2,
		lastError:  nil,
		retryCount: 0,
	}
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

				// Query for unprocessed events
				data, _, err := t.db.From("events").
					Select("*", "", false).
					Is("is_processed", "null").
					Order("block_number", &postgrest.OrderOpts{Ascending: true}).
					Order("log_index", &postgrest.OrderOpts{Ascending: true}).
					Limit(100, "").
					Execute()

				if err != nil {
					t.handleError(err)
					continue
				}

				var response []database.PublicEventsSelect
				if err := json.Unmarshal(data, &response); err != nil {
					t.handleError(err)
					continue
				}
				if len(response) == 0 {
					t.logger.Printf("No events found, waiting 15 seconds")
					time.Sleep(15 * time.Second)
					continue
				}

				t.logger.Printf("Received %d events, processing...", len(response))

				t.handleError(manualError("manual"))

				// Process each event
				for _, event := range response {
					t.logger.Printf("Processing event: %v", event)

					continue

					// Mark event as processed
					_, _, err = t.db.From("events").
						Update(map[string]interface{}{
							"is_processed": true,
						}, "", "").
						Eq("id", event.Id).
						Execute()

					if err != nil {
						t.handleError(err)
					}

					// Reset retry count on successful operation
					t.retryCount = 0
					t.lastError = nil

				}
			}
		}
	}()
	return nil
}

// handleError manages retry logic and backoff
func (t *Transformer) handleError(err error) {
	now := time.Now()

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
	time.Sleep(backoff)
}

// Stop gracefully stops the transformer
func (t *Transformer) Stop() {
	t.cancel()
	t.wg.Wait()
}
