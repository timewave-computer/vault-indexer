package indexer

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

type FinalityProcessor struct {
	logger       *logger.Logger
	ethClient    *ethclient.Client
	db           *supa.Client
	ctx          context.Context
	cancel       context.CancelFunc
	once         sync.Once
	wg           sync.WaitGroup
	reorgChannel chan ReorgEvent
}

type ReorgEvent struct {
	BlockNumber int64
	BlockHash   string
	BlockTag    string
}

func NewFinalityProcessor(ethClient *ethclient.Client, db *supa.Client) *FinalityProcessor {
	logger := logger.NewLogger("FinalityProcessor")
	ctx, cancel := context.WithCancel(context.Background())

	return &FinalityProcessor{
		logger:       logger,
		ethClient:    ethClient,
		db:           db,
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
		reorgChannel: make(chan ReorgEvent, 10),
	}
}

func (f *FinalityProcessor) Start() error {
	f.logger.Info("Finality processor started")
	errors := make(chan error, 10)

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case err := <-errors:
				f.logger.Error("Error in finality processor: %v", err)
				f.cancel()
				return
			case <-f.ctx.Done():
				// context cancelled, stop listening for errors
				return
			}
		}
	}()

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
						errors <- err
						return
					}

					currentBlockNumber := currentBlock.Number.Int64()

					f.logger.Info("Current %s block: %d", blockTag, currentBlockNumber)

					nearestIngestedEvent, err := GetNearestIngestedEvent(f.db, currentBlockNumber)
					if nearestIngestedEvent == nil {
						// no ingested events yet, wait for next iteration
						f.logger.Info("No ingested events yet, waiting for next iteration")
						time.Sleep(15 * time.Second)
						continue
					}
					f.logger.Info("Nearest ingested %v event: %v hash: %v", blockTag, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
					if err != nil {
						f.logger.Error("Error getting nearest ingested event: %v", err)
						errors <- err
						return
					}
					isCanonical, err := CheckCanonicalBlock(f.ethClient, f.logger, nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
					if err != nil {
						f.logger.Error("Error checking if nearest ingested event is canonical: %v", err)
						errors <- err
						return
					}

					if isCanonical {
						// update last validated block number
						err := f.updateLastValidatedBlockNumber(blockTag, nearestIngestedEvent.BlockNumber)
						if err != nil {
							f.logger.Error("Error updating last validated block number: %v", err)
							errors <- err
							return
						}
						continue
					} else {
						// raise hell
						f.logger.Error("Nearest ingested event does not match canonical block: %v, %v", nearestIngestedEvent.BlockNumber, nearestIngestedEvent.BlockHash)
						// Send reorg event to reorgChannel
						reorgEvent := ReorgEvent{
							BlockNumber: nearestIngestedEvent.BlockNumber,
							BlockHash:   nearestIngestedEvent.BlockHash,
							BlockTag:    blockTag,
						}
						select {
						case f.reorgChannel <- reorgEvent:
							f.logger.Info("Reorg event sent to channel: %+v", reorgEvent)
						default:
							f.logger.Warn("Reorg channel is full, dropping event")
						}
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

func (f *FinalityProcessor) getLastValidatedBlockNumber(blockTag string) (database.PublicBlockFinalitySelect, error) {
	var blockFinality []database.PublicBlockFinalitySelect
	_, err := f.db.From("block_finality").Select("last_validated_block_number", "", false).Eq("block_tag", blockTag).ExecuteTo(&blockFinality)
	if err != nil {
		return database.PublicBlockFinalitySelect{}, err
	}

	if len(blockFinality) == 0 {
		return database.PublicBlockFinalitySelect{
			BlockTag:                 blockTag,
			LastValidatedBlockNumber: 0,
		}, nil
	}

	return blockFinality[0], nil
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

func (f *FinalityProcessor) ReorgChannel() <-chan ReorgEvent {
	return f.reorgChannel
}
