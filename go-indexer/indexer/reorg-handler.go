package indexer

import (
	"fmt"
	"strconv"
	"time"

	"github.com/supabase-community/postgrest-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

func (i *Indexer) handleReorg(reorgEvent ReorgEvent) {
	i.logger.Info("Handling reorg event: %+v", reorgEvent)

	// 1. Stop all child processes using shared halt manager
	i.logger.Info("Halting all child processes due to reorg...")
	i.haltManager.Halt()

	// Wait for all processors to halt
	isHalted := i.haltManager.WaitHalted(120 * time.Second)
	if !isHalted {
		i.logger.Error("Processors did not halt in time")
		i.Stop()
		return
	}
	i.logger.Info("All processors halted")

	// 2. Clear the event queue
	i.logger.Info("Clearing event queue due to reorg...")
	i.eventQueue.Clear()

	// 3. Stop the event processor
	i.logger.Info("Stopping event processor due to reorg...")

	// 3. Execute cleanup logic
	i.logger.Info("Finding last consistent block before block number: %d", reorgEvent.BlockNumber)
	lastConsistentBlock, err := i.findLastConsistentBlock(reorgEvent.BlockNumber)
	if err != nil {
		i.logger.Error("Failed to find last consistent block: %v", err)
		i.Stop()
		return
	}
	i.logger.Info("Executing cleanup logic after last consistent block number: %d", lastConsistentBlock)
	err = i.transformer.handleCleanupFromBlock(lastConsistentBlock, i.postgresClient)
	if err != nil {
		i.logger.Error("Failed to cleanup reorg data: %v", err)
		i.Stop()
		return
	}

	i.transformer.Stop()

	// 4. Power down the indexer
	i.logger.Info("Powering down indexer after reorg handling...")
	i.Stop()
}

// findLastConsistentBlock finds the last block that is still valid by checking the canonical chain
func (i *Indexer) findLastConsistentBlock(reorgDetectedBlock int64) (int64, error) {
	i.logger.Debug("Searching for last valid block before %d", reorgDetectedBlock)

	fromBlock := reorgDetectedBlock - 1
	for {
		nearestIngestedEvent, err := i.getNearestIngestedEvent(fromBlock)
		if err != nil {
			return 0, err
		}
		if nearestIngestedEvent == nil {
			return 0, fmt.Errorf("no nearest ingested event found for block %d", fromBlock)
		}

		isCanonical, err := CheckCanonicalBlock(i.ethClient, i.logger, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
		i.logger.Debug("Is canonical: %v, block number: %v, hash: %v", isCanonical, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
		if err != nil {
			return 0, err
		}

		if isCanonical {
			return nearestIngestedEvent.BlockNumber, nil
		}

		fromBlock = nearestIngestedEvent.BlockNumber - 1
	}

}

// GetNearestIngestedEvent retrieves the nearest ingested event for a given block number
func (r *Indexer) getNearestIngestedEvent(blockNumber int64) (*database.PublicEventsSelect, error) {
	var nearestEvents []database.PublicEventsSelect

	_, err := r.supabaseClient.From("events").Select("block_number, block_hash", "", false).
		Lte("block_number", strconv.FormatInt(blockNumber, 10)).
		Limit(1, "").
		Order("block_number", &postgrest.OrderOpts{Ascending: false}).
		ExecuteTo(&nearestEvents)

	if err != nil {
		return nil, err
	}
	if len(nearestEvents) == 0 {
		return nil, nil
	}

	return &nearestEvents[0], nil

}
