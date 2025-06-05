package indexer

import (
	"context"
	"encoding/json"
	"sync"
	"time"

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
}

// NewTransformer creates a new transformer instance
func NewTransformer(db *supa.Client) *Transformer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Transformer{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
		logger: NewLogger("Transformer"),
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
					Neq("is_processed", "true").
					Order("block_number", &postgrest.OrderOpts{Ascending: true}).
					Order("log_index", &postgrest.OrderOpts{Ascending: true}).
					Limit(100, "").
					Execute()

				if err != nil {
					t.logger.Printf("Error querying events: %v", err)
					time.Sleep(15 * time.Second)
					continue
				}

				var response []database.PublicEventsSelect
				if err := json.Unmarshal(data, &response); err != nil {
					t.logger.Printf("Error unmarshaling events: %v", err)
					continue
				}
				if len(response) == 0 {
					t.logger.Printf("No events found, waiting 15 seconds")
					time.Sleep(15 * time.Second)
					continue
				}

				t.logger.Printf("Received %d events, processing...", len(response))

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
						t.logger.Printf("Error marking event as processed: %v", err)
					}
				}
			}
		}
	}()
	return nil
}

// Stop gracefully stops the transformer
func (t *Transformer) Stop() {
	t.cancel()
	t.wg.Wait()
}
