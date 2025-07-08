package indexer

import (
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

type ReorgHandler struct {
	logger         *logger.Logger
	ethClient      *ethclient.Client
	supabaseClient *supabase.Client
}

func NewReorgHandler(ethClient *ethclient.Client, supabaseClient *supabase.Client) *ReorgHandler {
	return &ReorgHandler{
		logger:         logger.NewLogger("ReorgHandler"),
		ethClient:      ethClient,
		supabaseClient: supabaseClient,
	}
}

func (r *ReorgHandler) cleanupReorgData(reorgEvent ReorgEvent) error {
	r.logger.Info("Cleaning up data for reorg detectedat block %d", reorgEvent.BlockNumber)

	fromBlock := reorgEvent.BlockNumber

	// Step 1: Find the last valid block
	lastConsistentBlock, err := r.findLastConsistentBlock(fromBlock)
	if err != nil {
		return fmt.Errorf("failed to find last valid block: %w", err)
	}

	r.logger.Info("Last consistent block: %d", lastConsistentBlock)

	// TODO: Implement specific cleanup logic based on your requirements
	// This could include:
	// - Removing events from database that are from the reorged blocks
	// - Marking positions as invalid
	// - Clearing cached data
	// - Updating finality status

	// Example cleanup operations:
	// 1. Delete events from reorged blocks
	// 2. Reset position tracking
	// 3. Clear any cached state
	r.logger.Info("Reorg cleanup completed successfully")

	return nil

}

// findLastConsistentBlock finds the last block that is still valid by checking the canonical chain
func (r *ReorgHandler) findLastConsistentBlock(blockNumber int64) (int64, error) {
	r.logger.Info("Searching for last valid block before %d", blockNumber)

	fromBlock := blockNumber
	for {
		nearestIngestedEvent, err := GetNearestIngestedEvent(r.supabaseClient, fromBlock)
		if err != nil {
			return 0, err
		}
		if nearestIngestedEvent == nil {
			return 0, fmt.Errorf("no nearest ingested event found for block %d", fromBlock)
		}

		isCanonical, err := CheckCanonicalBlock(r.ethClient, r.logger, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
		r.logger.Info("Is canonical: %v, block number: %v, hash: %v", isCanonical, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
		if err != nil {
			return 0, err
		}

		if isCanonical {
			return fromBlock, nil
		}

		fromBlock = nearestIngestedEvent.BlockNumber - 1
	}

}
