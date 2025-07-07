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

// EventProcessor handles processing of events from the event queue
type EventProcessor struct {
	eventQueue            *event_queue.EventQueue
	extractor             *Extractor
	ethClient             *ethclient.Client
	wg                    sync.WaitGroup
	logger                *logger.Logger
	requiredConfirmations uint64
	ctx                   context.Context
	stopChan              chan StopChannel
}

// NewEventProcessor creates a new event processor instance
func NewEventProcessor(eventQueue *event_queue.EventQueue, extractor *Extractor, ethClient *ethclient.Client, requiredConfirmations uint64, ctx context.Context, stopChan chan StopChannel) *EventProcessor {

	return &EventProcessor{
		eventQueue:            eventQueue,
		extractor:             extractor,
		ethClient:             ethClient,
		logger:                logger.NewLogger("EventProcessor"),
		requiredConfirmations: requiredConfirmations,
		ctx:                   ctx,
		stopChan:              stopChan,
	}
}

// Start begins the event processing
func (ep *EventProcessor) Start() error {
	ep.logger.Info("Event processor started")

	errorChan := make(chan error)

	// Main processing goroutine
	ep.wg.Add(1)
	go func() {
		defer ep.wg.Done()
		for {
			select {
			case err := <-errorChan:
				ep.logger.Error("Error in event queue processor: %v", err)
				ep.stopChan <- StopChannel{
					isReorg:     false,
					blockNumber: 0,
					blockHash:   "",
				}
				return
			case <-ep.stopChan:
				ep.logger.Info("Stop signal received, stopping event processor")
				return
			case <-ep.ctx.Done():
				ep.logger.Info("Context cancelled, stopping event processor")
				// context cancelled, stop processing events
				return
			default:
				// continue
			}

			event := ep.eventQueue.Next()

			if event == nil {
				ep.logger.Info("No events in ingestion queue, waiting 15 seconds")
				select {
				case <-ep.stopChan:
					ep.logger.Info("Stop signal received, stopping event processor")
					return
				case <-time.After(15 * time.Second):
				case <-ep.ctx.Done():
					return
				}
				continue
			}
			ep.logger.Info("Processing event: %v", event.BlockHash)

			currentBlock, err := ep.ethClient.BlockNumber(ep.ctx)
			if err != nil {
				select {
				case errorChan <- fmt.Errorf("failed to get current block number: %w", err):
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
				case err := <-errorChan:
					ep.logger.Error("Error in event queue processor: %v", err)
					return
				case <-ep.stopChan:
					ep.logger.Info("Stop signal received, stopping event processor")
					return
				case <-ep.ctx.Done():
					ep.logger.Info("Context cancelled, stopping event processor")
					return
				}
				continue
			}

			err = ep.extractor.writeIdempotentEvent(event.Data, event.Event)
			if err != nil {
				select {
				case errorChan <- fmt.Errorf("failed to extract event %s, %v", err, event):
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
