package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// CheckCanonicalBlock fetches a block by hash and compares its block number
func CheckCanonicalBlock(ethClient *ethclient.Client, logger *logger.Logger, blockNumber int64, blockHash string) (bool, error) {
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	logger.Debug("Checking canonical block for %v (hex: %s) by hash %v", blockNumber, blockNumberHex, blockHash)

	header, err := ethClient.HeaderByHash(context.Background(), common.HexToHash(blockHash))

	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			logger.Debug("Header not found for block %v with hash %v", blockNumber, blockHash)
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
