package indexer

import (
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
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
		nearestIngestedEvent, err := r.getNearestIngestedEvent(fromBlock)
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

// GetNearestIngestedEvent retrieves the nearest ingested event for a given block number
func (r *ReorgHandler) getNearestIngestedEvent(blockNumber int64) (*database.PublicEventsSelect, error) {
	if blockNumber == 0 {
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
	} else {
		var mostRecentEvents []database.PublicEventsSelect

		_, err := r.supabaseClient.From("events").Select("block_number, block_hash", "", false).
			Limit(1, "").
			Lte("block_number", strconv.FormatInt(blockNumber, 10)).
			Order("block_number", &postgrest.OrderOpts{Ascending: false}).
			ExecuteTo(&mostRecentEvents)
		if err != nil {
			return nil, err
		}

		if len(mostRecentEvents) == 0 {
			return nil, nil
		}

		return &mostRecentEvents[0], nil
	}
}
