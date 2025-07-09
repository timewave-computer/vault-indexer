package indexer

import (
	"fmt"
	"time"
)

func (i *Indexer) handleReorg(reorgEvent ReorgEvent) {
	i.logger.Info("Handling reorg event: %+v", reorgEvent)

	// 1. Stop all child processes
	i.logger.Info("Stopping all child processes due to reorg...")
	i.extractor.Stop()
	// must halt so it stops processing, but use the methods to update the database
	i.transformer.Halt()
	isHalted := i.transformer.WaitHalted(120 * time.Second)
	if !isHalted {
		i.logger.Error("Transformer did not pause in time")
		i.Stop()
		return
	}

	i.logger.Info("Transformer halted", isHalted)
	i.finalityProcessor.Stop()

	// 2. Clear the event queue
	i.logger.Info("Clearing event queue due to reorg...")
	i.eventQueue.Clear()

	// 3. Stop the event processor
	i.logger.Info("Stopping event processor due to reorg...")
	i.eventProcessor.Stop()

	// 3. Execute cleanup logic
	i.logger.Info("Finding last consistent block before block number: %d", reorgEvent.BlockNumber)
	lastConsistentBlock, err := i.findLastConsistentBlock(reorgEvent.BlockNumber)
	if err != nil {
		i.logger.Error("Failed to find last consistent block: %v", err)
		i.Stop()
		return
	}
	i.logger.Info("Executing cleanup logic after block number: %d", lastConsistentBlock)
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
func (i *Indexer) findLastConsistentBlock(blockNumber int64) (int64, error) {
	i.logger.Debug("Searching for last valid block before %d", blockNumber)

	fromBlock := blockNumber
	for {
		nearestIngestedEvent, err := GetNearestIngestedEvent(i.supabaseClient, fromBlock)
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
