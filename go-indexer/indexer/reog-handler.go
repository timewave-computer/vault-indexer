package indexer

func (i *Indexer) handleReorg(reorgEvent ReorgEvent) {
	i.logger.Info("Handling reorg event: %+v", reorgEvent)

	// 1. Stop all child processes
	i.logger.Info("Stopping all child processes due to reorg...")
	i.extractor.Stop()
	i.transformer.Stop()
	i.finalityProcessor.Stop()

	// 2. Clear the event queue
	i.logger.Info("Clearing event queue due to reorg...")
	i.eventQueue.Clear()

	// 3. Stop the event processor
	i.logger.Info("Stopping event processor due to reorg...")
	i.eventProcessor.Stop()

	// 3. Execute cleanup logic
	i.logger.Info("Executing reorg cleanup logic...")
	if err := i.transformer.handleCleanupFromBlock(22739105, i.postgresClient); err != nil {
		i.logger.Error("Failed to cleanup reorg data: %v", err)
		i.Stop()
		return
	}

	// 4. Power down the indexer
	i.logger.Info("Powering down indexer after reorg handling...")
	i.Stop()
}
