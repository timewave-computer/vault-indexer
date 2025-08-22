package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	event_queue "github.com/timewave/vault-indexer/go-indexer/event-ingestion-queue"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

const EVENT_PROCESSOR_POLL_INTERVAL = 500 * time.Millisecond

// EventProcessor handles processing of events from the event queue
type EventProcessor struct {
	eventQueue            *event_queue.EventQueue
	extractor             *Extractor
	ethClient             *ethclient.Client
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	logger                *logger.Logger
	requiredConfirmations uint64
	haltManager           *HaltManager
}

// NewEventProcessor creates a new event processor instance
func NewEventProcessor(eventQueue *event_queue.EventQueue, extractor *Extractor, ethClient *ethclient.Client, requiredConfirmations uint64, ctx context.Context, haltManager *HaltManager) *EventProcessor {
	ctx, cancel := context.WithCancel(ctx)
	return &EventProcessor{
		eventQueue:            eventQueue,
		extractor:             extractor,
		ethClient:             ethClient,
		ctx:                   ctx,
		cancel:                cancel,
		logger:                logger.NewLogger("EventProcessor"),
		requiredConfirmations: requiredConfirmations,
		haltManager:           haltManager,
	}
}

// Start begins the event processing
func (ep *EventProcessor) Start() error {
	ep.logger.Info("Event processor started")

	errors := make(chan error, 10)

	// Error handling goroutine
	ep.wg.Add(1)
	go func() {
		defer ep.wg.Done()
		for {
			select {
			case err := <-errors:
				ep.logger.Error("Error in event queue processor: %v", err)
				ep.Stop()
				return
			case <-ep.ctx.Done():
				// context cancelled, stop listening for errors
				return

			}
		}
	}()

	// Main processing goroutine
	ep.wg.Add(1)
	go func() {
		defer ep.wg.Done()
		for {
			select {
			case <-ep.ctx.Done():
				// context cancelled, stop processing events
				return
			case <-ep.haltManager.HaltChannel():
				// halt requested, stop processing events
				ep.logger.Info("Halted, exiting cycle")
				return
			default:
				// continue
			}

			event := ep.eventQueue.Next()

			if event == nil {
				select {
				case <-time.After(EVENT_PROCESSOR_POLL_INTERVAL):
				case <-ep.ctx.Done():
					return

				}
				continue
			}
			ep.logger.Info("Processing event: %v", event.BlockHash)

			currentBlock, err := ep.ethClient.BlockNumber(ep.ctx)
			if err != nil {
				select {
				case errors <- fmt.Errorf("failed to get current block number: %w", err):
				default:
					// Channel is closed or full, stop processing
					return
				}
				continue
			}
			ep.logger.Info("Ingestion queue size: %d, current block: %d", ep.eventQueue.Len(), currentBlock)

			if (event.BlockNumber + ep.requiredConfirmations) > currentBlock {
				// not enough confirmations, put back in queue
				ep.logger.Info("Not enough confirmations, putting back in queue and waiting %v. Event name: %s, block %d, current block %d, required height for ingestion: %d",
					EVENT_PROCESSOR_POLL_INTERVAL,
					event.Event.Name, event.BlockNumber, currentBlock, event.BlockNumber+ep.requiredConfirmations)
				ep.eventQueue.Insert(*event)
				select {
				case <-time.After(EVENT_PROCESSOR_POLL_INTERVAL):
				case <-ep.ctx.Done():
					return
				}
				continue
			}

			err = ep.extractor.writeIdempotentEvent(event.Data, event.Event)
			if err != nil {
				select {
				case errors <- fmt.Errorf("failed to extract event %s, %v", err, event):
				default:
					// Channel is closed or full, stop processing
					return
				}
				continue
			}

		}
	}()

	return nil
}

// Stop gracefully stops the event processor
func (ep *EventProcessor) Stop() {
	ep.logger.Info("Stopping event processor...")

	// Cancel context
	ep.cancel()

	// Wait for all goroutines to finish
	ep.wg.Wait()

	ep.logger.Info("Event processor stopped")

}
