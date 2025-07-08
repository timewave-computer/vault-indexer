package indexer

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// GetNearestIngestedEvent retrieves the nearest ingested event for a given block number
func GetNearestIngestedEvent(db *supa.Client, blockNumber int64) (*database.PublicEventsSelect, error) {
	if blockNumber == 0 {
		var nearestEvents []database.PublicEventsSelect

		_, err := db.From("events").Select("block_number, block_hash", "", false).
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

		_, err := db.From("events").Select("block_number, block_hash", "", false).
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

// CheckCanonicalBlock fetches a block by hash and compares its block number
func CheckCanonicalBlock(ethClient *ethclient.Client, logger *logger.Logger, blockNumber int64, blockHash string) (bool, error) {
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	logger.Debug("Checking canonical block for %v (hex: %s) by hash %v", blockNumber, blockNumberHex, blockHash)

	header, err := ethClient.HeaderByHash(context.Background(), common.HexToHash(blockHash))

	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			logger.Error("Header not found for block %v with hash %v", blockNumber, blockHash)
			return false, nil
		}
		logger.Error("Error getting canonical block for %v: %v", blockNumber, err)
		return false, err
	}
	logger.Info("Canonical block number for hash %v: %v", blockHash, header.Number.Int64())

	isMatch := blockNumber == header.Number.Int64()
	logger.Debug("Is match: %v, event block number: %v, header block number: %v", isMatch, blockNumber, header.Number.Int64())

	return isMatch, nil
}
