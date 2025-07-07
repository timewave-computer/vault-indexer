package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	event_queue "github.com/timewave/vault-indexer/go-indexer/event-ingestion-queue"
	"github.com/timewave/vault-indexer/go-indexer/health"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// EventProcessor handles processing of events from the event queue
type EventProcessor struct {
	eventQueue            *event_queue.EventQueue
	extractor             *Extractor
	ethClient             *ethclient.Client
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	logger                *logger.Logger
	healthServer          *health.Server
	requiredConfirmations uint64
	stopChan              chan struct{}
	stopOnce              sync.Once
}

// NewEventProcessor creates a new event processor instance
func NewEventProcessor(eventQueue *event_queue.EventQueue, extractor *Extractor, ethClient *ethclient.Client, requiredConfirmations uint64) *EventProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	// Create health check server
	healthServer := health.NewServer(8082, "event_processor") // Different port for event processor health checks

	return &EventProcessor{
		eventQueue:            eventQueue,
		extractor:             extractor,
		ethClient:             ethClient,
		ctx:                   ctx,
		cancel:                cancel,
		logger:                logger.NewLogger("EventProcessor"),
		healthServer:          healthServer,
		requiredConfirmations: requiredConfirmations,
		stopChan:              make(chan struct{}),
	}
}

// Start begins the event processing
func (ep *EventProcessor) Start() error {
	ep.logger.Info("Event processor started")

	// Start health check server
	if err := ep.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	errors := make(chan error, 10)

	// Error handling goroutine
	ep.wg.Add(1)
	go func() {
		defer ep.wg.Done()
		for {
			select {
			case err := <-errors:
				ep.logger.Error("Error in event queue processor: %v", err)
				ep.healthServer.SetStatus("unhealthy")
				ep.Stop()
				return
			case <-ep.ctx.Done():
				// context cancelled, stop listening for errors
				return
			case <-ep.stopChan:
				// external stop signal received
				ep.logger.Info("External stop signal received, shutting down error handler")
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
			case <-ep.stopChan:
				// external stop signal received, stop processing events
				ep.logger.Info("External stop signal received, stopping event processing")
				return
			default:
				// continue
			}

			event := ep.eventQueue.Next()

			if event == nil {
				ep.logger.Info("No events in ingestion queue, waiting 15 seconds")
				select {
				case <-time.After(15 * time.Second):
				case <-ep.ctx.Done():
					return
				case <-ep.stopChan:
					ep.logger.Info("External stop signal received during wait")
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
				ep.logger.Info("Not enough confirmations, putting back in queue and waiting 15 seconds. Event name: %s, block %d, current block %d, required height for ingestion: %d",
					event.Event.Name, event.BlockNumber, currentBlock, event.BlockNumber+ep.requiredConfirmations)
				ep.eventQueue.Insert(*event)
				select {
				case <-time.After(15 * time.Second):
				case <-ep.ctx.Done():
					return
				case <-ep.stopChan:
					ep.logger.Info("External stop signal received during confirmation wait")
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

			// Set health status to healthy after successful processing
			ep.healthServer.SetStatus("healthy")
		}
	}()

	return nil
}

// Stop gracefully stops the event processor
func (ep *EventProcessor) Stop() {
	ep.logger.Info("Stopping event processor...")

	// Set health status to unhealthy
	ep.healthServer.SetStatus("unhealthy")

	// Close stop channel to signal shutdown (only once)
	ep.stopOnce.Do(func() {
		close(ep.stopChan)
	})

	// Cancel context
	ep.cancel()

	// Wait for all goroutines to finish
	ep.wg.Wait()

	// Stop health check server
	if err := ep.healthServer.Stop(); err != nil {
		ep.logger.Error("Error stopping health check server: %v", err)
	}

	ep.logger.Info("Event processor stopped")
}

// StopProcessor sends a stop signal to the event processor
// This method is safe to call multiple times and from multiple goroutines
func (ep *EventProcessor) StopProcessor() {
	ep.logger.Info("External stop signal sent - stopping event processor")

	// Use a recover mechanism to handle the case where the channel is already closed
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Channel is already closed, this is expected if Stop() was called
				ep.logger.Info("Stop channel already closed, ignoring stop signal")
			}
		}()

		select {
		case ep.stopChan <- struct{}{}:
			// Signal sent successfully
		default:
			// Channel is full, ignore
		}
	}()
}
