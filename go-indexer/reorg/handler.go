package reorg

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/config"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// ReorgError represents a blockchain reorganization error that requires special handling
type ReorgError struct {
	BlockNumber int64
	BlockHash   string
	Message     string
}

func (e *ReorgError) Error() string {
	return fmt.Sprintf("reorg detected at block %d (hash: %s): %s", e.BlockNumber, e.BlockHash, e.Message)
}

// IsReorgError checks if an error is a reorganization error
func IsReorgError(err error) bool {
	_, ok := err.(*ReorgError)
	return ok
}

// NewReorgError creates a new ReorgError
func NewReorgError(blockNumber int64, blockHash string, message string) *ReorgError {
	return &ReorgError{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		Message:     message,
	}
}

// Handler handles blockchain reorganization cleanup
type Handler struct {
	logger         *logger.Logger
	supabaseClient *supa.Client
	postgresClient *sql.DB
	ethClient      *ethclient.Client
	config         *config.Config
}

// NewHandler creates a new reorg handler
func NewHandler(supabaseClient *supa.Client, postgresClient *sql.DB, ethClient *ethclient.Client, config *config.Config) *Handler {
	return &Handler{
		logger:         logger.NewLogger("ReorgHandler"),
		supabaseClient: supabaseClient,
		postgresClient: postgresClient,
		ethClient:      ethClient,
		config:         config,
	}
}

// HandleReorg performs the complete reorg cleanup process
func (h *Handler) HandleReorg(reorgErr *ReorgError) error {
	h.logger.Info("Starting reorg cleanup for block %d hash: %s", reorgErr.BlockNumber, reorgErr.BlockHash)

	fromBlock := reorgErr.BlockNumber

	// Step 1: Find the last valid block
	lastConsistentBlock, err := h.findLastConsistentBlock(fromBlock)
	if err != nil {
		return fmt.Errorf("failed to find last valid block: %w", err)
	}

	h.logger.Info("Last consistent block: %d", lastConsistentBlock)

	h.logger.Info("Reorg cleanup completed successfully")
	return nil
}

// findLastValidBlock finds the last block that is still valid by checking the canonical chain
func (h *Handler) findLastConsistentBlock(blockNumber int64) (int64, error) {
	h.logger.Info("Searching for last valid block before %d", blockNumber)

	fromBlock := blockNumber
	for {
		nearestIngestedEvent, err := h.getNearestIngestedEvent(fromBlock)
		if err != nil {
			return 0, err
		}
		if nearestIngestedEvent == nil {
			return 0, fmt.Errorf("no nearest ingested event found for block %d", fromBlock)
		}

		isCanonical, err := h.checkCanonicalBlock(nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
		h.logger.Info("Is canonical: %v, block number: %v, hash: %v", isCanonical, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
		if err != nil {
			return 0, err
		}

		if isCanonical {
			return fromBlock, nil
		}

		fromBlock = nearestIngestedEvent.BlockNumber - 1

	}

}

func (h *Handler) getNearestIngestedEvent(blockNumber int64) (*database.PublicEventsSelect, error) {

	var mostRecentEvents []database.PublicEventsSelect

	_, err := h.supabaseClient.From("events").Select("block_number, block_hash", "", false).
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

// fetches by block hash and compares block number
func (h *Handler) checkCanonicalBlock(blockNumber int64, blockHash string) (bool, error) {
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	h.logger.Debug("Checking canonical block for %v (hex: %s) by hash %v", blockNumber, blockNumberHex, blockHash)

	header, err := h.ethClient.HeaderByHash(context.Background(), common.HexToHash(blockHash))

	if err != nil {
		h.logger.Error("Error getting canonical block for %v: %v", blockNumber, err)
		if err.Error() == "not found" {
			// this implies that the block is not canonical
			return false, nil
		}
		return false, err
	}
	h.logger.Info("Block number: %v, Canonical block number for hash %v: %v", blockNumber, blockHash, header.Number.Int64())

	isMatch := blockNumber == header.Number.Int64()
	h.logger.Debug("Is match: %v, event block number: %v, header block number: %v", isMatch, blockNumber, header.Number.Int64())

	return isMatch, nil
}
