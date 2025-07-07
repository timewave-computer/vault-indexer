package indexer

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
	"github.com/timewave/vault-indexer/go-indexer/reorg"
)

type FinalityProcessor struct {
	logger    *logger.Logger
	ethClient *ethclient.Client
	db        *supa.Client
	ctx       context.Context
	cancel    context.CancelFunc
	once      sync.Once
	wg        sync.WaitGroup
	errors    chan error
}

func NewFinalityProcessor(ethClient *ethclient.Client, db *supa.Client, errorChan chan error) *FinalityProcessor {
	logger := logger.NewLogger("FinalityProcessor")
	ctx, cancel := context.WithCancel(context.Background())

	return &FinalityProcessor{
		logger:    logger,
		ethClient: ethClient,
		db:        db,
		ctx:       ctx,
		cancel:    cancel,
		wg:        sync.WaitGroup{},
		errors:    errorChan,
	}
}

func (f *FinalityProcessor) Start() error {
	f.logger.Info("Finality processor started")

	go func() {

		for {

			select {
			case <-f.ctx.Done():
				return
			default:

				blockTags := []string{"finalized", "safe"}
				blockNumbers := map[string]int64{
					"finalized": int64(rpc.FinalizedBlockNumber),
					"safe":      int64(rpc.SafeBlockNumber),
				}

				for _, blockTag := range blockTags {
					currentBlock, err := f.ethClient.HeaderByNumber(context.Background(), big.NewInt(blockNumbers[blockTag]))
					if err != nil {
						f.logger.Error("Error getting last %s block: %v", blockTag, err)
						f.errors <- err
						return
					}

					currentBlockNumber := currentBlock.Number.Int64()

					f.logger.Info("Current %s block: %d", blockTag, currentBlockNumber)

					nearestIngestedEvent, err := f.getNearestIngestedEvent(currentBlockNumber)
					if nearestIngestedEvent == nil {
						// no ingested events yet, wait for next iteration
						f.logger.Info("No ingested events yet, waiting for next iteration")
						time.Sleep(15 * time.Second)
						continue
					}
					f.logger.Info("Nearest ingested %v event id: %v, block number: %v, hash: %v", blockTag, nearestIngestedEvent.Id, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
					if err != nil {
						f.logger.Error("Error getting nearest ingested event: %v", err)
						f.errors <- err
						return
					}
					isCanonical, err := f.checkCanonicalBlock(nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash, nearestIngestedEvent.Id)
					if err != nil {
						f.logger.Error("Error checking if nearest ingested event is canonical: %v", err)
						f.errors <- err
						return
					}

					if isCanonical {
						// update last validated block number
						err := f.updateLastValidatedBlockNumber(blockTag, nearestIngestedEvent.BlockNumber)
						if err != nil {
							f.logger.Error("Error updating last validated block number: %v", err)
							f.errors <- err
							return
						}
						continue
					} else {
						// raise hell
						f.logger.Error("Nearest ingested event does not match canonical block: %v, %v", nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
						// TODO: trigger re-org. (stops all processes, locates last valid event, starts from there)
						reorgErr := reorg.NewReorgError(
							nearestIngestedEvent.BlockNumber,
							nearestIngestedEvent.BlockHash,
							"nearest ingested event does not match canonical block",
						)
						f.errors <- reorgErr
						return
					}
				}
				time.Sleep(15 * time.Second)

			}
		}
	}()

	return nil
}

func (f *FinalityProcessor) Stop() {
	f.once.Do(func() {
		f.logger.Info("Stopping finality processor...")
		f.cancel()
	})
}

func (f *FinalityProcessor) getNearestIngestedEvent(blockNumber int64) (*database.PublicEventsSelect, error) {

	if blockNumber == 0 {
		var nearestEvents []database.PublicEventsSelect

		_, err := f.db.From("events").Select("block_number, block_hash, id", "", false).
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

		_, err := f.db.From("events").Select("block_number, block_hash, id", "", false).
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

// fetches by block hash and compares block number
func (f *FinalityProcessor) checkCanonicalBlock(blockNumber int64, blockHash string, eventId string) (bool, error) {
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	f.logger.Debug("Checking canonical block for %v (hex: %s) by hash %v", blockNumber, blockNumberHex, blockHash)

	header, err := f.ethClient.HeaderByHash(context.Background(), common.HexToHash(blockHash))

	if err != nil {
		if err.Error() == "not found" {
			// this implies that the block is not canonical
			return false, nil
		} else {
			f.logger.Error("Error getting canonical block for %v: %v", blockNumber, err)
			return false, err
		}

	}
	f.logger.Info("Canonical block event id %v number for hash %v: %v", eventId, blockHash, header.Number.Int64())

	isMatch := blockNumber == header.Number.Int64()
	f.logger.Debug("Is match: %v, event block number: %v, header block number: %v", isMatch, blockNumber, header.Number.Int64())

	return isMatch, nil
}

func (f *FinalityProcessor) updateLastValidatedBlockNumber(blockTag string, blockNumber int64) error {

	f.logger.Info("Setting last validated %v block to %v", blockTag, blockNumber)

	_, _, err := f.db.From("block_finality").Upsert(
		database.ToBlockFinalityUpsert(database.PublicBlockFinalityUpdate{
			BlockTag:                 &blockTag,
			LastValidatedBlockNumber: &blockNumber,
		}), "", "", "",
	).Eq("block_tag", blockTag).Execute()
	if err != nil {
		return err
	}
	return nil
}
