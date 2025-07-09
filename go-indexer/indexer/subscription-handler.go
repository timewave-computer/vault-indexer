package indexer

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	event_queue "github.com/timewave/vault-indexer/go-indexer/event-ingestion-queue"
)

func (i *Indexer) setupSubscriptions(wg *sync.WaitGroup) error {

	for _, contract := range i.config.Contracts {
		parsedABI, err := i.loadAbi(contract)
		if err != nil {
			return fmt.Errorf("failed to load ABI for contract %s: %w", contract.Address, err)
		}
		contractAddress := common.HexToAddress(contract.Address)

		for _, _eventName := range contract.Events {
			eventName := parsedABI.Events[_eventName]

			wg.Add(1)
			go func(event abi.Event) {
				// Signal that this subscription goroutine has started
				wg.Done()

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
						// Use centralized error channel
						select {
						case i.eventIngestionErrorChan <- fmt.Errorf("failed to subscribe to event %s for contract %s: %w", event.Name, contract.Address, err):
						default:
							// Channel is closed or full, ignore
						}
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

	return nil
}

func (i *Indexer) subscribeToEvent(contractAddress common.Address, event abi.Event) error {
	i.logger.Debug("Subscribing to %s on contract %s", event.Name, contractAddress.String())

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
			i.logger.Info("Received log: %v", vLog)
			eventLog := event_queue.EventLog{
				BlockNumber:     vLog.BlockNumber,
				LogIndex:        vLog.Index,
				Event:           event,
				Data:            vLog,
				ContractAddress: contractAddress,
				BlockHash:       vLog.BlockHash,
			}
			i.eventQueue.Insert(eventLog)
			i.logger.Info("Event inserted into sorted queue: block %d, log index %d", vLog.BlockNumber, vLog.Index)
		case <-i.ctx.Done():
			sub.Unsubscribe()
			return nil
		}
	}
}
