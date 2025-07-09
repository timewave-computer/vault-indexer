package indexer

import (
	"context"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/supabase-community/postgrest-go"
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
	haltManager  *HaltManager
}

type ReorgEvent struct {
	BlockNumber int64
	BlockHash   string
	BlockTag    string
}

func NewFinalityProcessor(ethClient *ethclient.Client, db *supa.Client, ctx context.Context, haltManager *HaltManager, reorgChan chan ReorgEvent) *FinalityProcessor {
	logger := logger.NewLogger("FinalityProcessor")
	ctx, cancel := context.WithCancel(ctx)

	return &FinalityProcessor{
		logger:       logger,
		ethClient:    ethClient,
		db:           db,
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
		reorgChannel: reorgChan,
		haltManager:  haltManager,
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
				f.Stop()
				return
			case <-f.ctx.Done():
				// context cancelled, stop listening for errors
				return
			}
		}
	}()

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case <-f.ctx.Done():
				return
			case <-f.reorgChannel:
				// reorg detected, stop processing
				f.logger.Info("Reorg detected, exiting cycle")
				return
			case <-f.haltManager.HaltChannel():
				// halt requested, stop processing
				f.logger.Info("Halted, exiting cycle")
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

					lastValidatedBlockNumber, err := f.getLastValidatedBlockNumber(blockTag)
					if err != nil {
						f.logger.Error("Error getting last validated block number: %v", err)
						errors <- err
						return
					}

					event, err := f.getNextEventToValidate(lastValidatedBlockNumber)
					if err != nil {
						f.logger.Error("Error getting next event to validate: %v", err)
						errors <- err
						return
					}
					if event == nil {
						// no next event to validate, wait for next iteration
						f.logger.Info("No next event to validate, waiting 15 seconds for next iteration")
						time.Sleep(15 * time.Second)
						continue
					}
					f.logger.Debug("Next event to validate: %v", event)
					if event.BlockNumber < lastValidatedBlockNumber {
						f.logger.Info("Next event to validate is at block number %v, which is less than last validated block number %v, waiting 15 seconds for next iteration", event.BlockNumber, lastValidatedBlockNumber)
						time.Sleep(15 * time.Second)
						continue
					}

					isCanonical, err := CheckCanonicalBlock(f.ethClient, f.logger, event.BlockNumber, event.BlockHash)
					if err != nil {
						f.logger.Error("Error checking if nearest ingested event is canonical: %v", err)
						errors <- err
						return
					}

					if isCanonical {
						// update last validated block number
						err := f.updateLastValidatedBlockNumber(blockTag, event.BlockNumber)
						if err != nil {
							f.logger.Error("Error updating last validated block number: %v", err)
							errors <- err
							return
						}
						continue
					} else {
						// raise hell
						f.logger.Error("Nearest ingested event does not match canonical block: %v, %v", event.BlockNumber, event.BlockHash)
						// Send reorg event to reorgChannel
						reorgEvent := ReorgEvent{
							BlockNumber: event.BlockNumber,
							BlockHash:   event.BlockHash,
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

func (f *FinalityProcessor) getLastValidatedBlockNumber(blockTag string) (int64, error) {
	var blockFinality []database.PublicBlockFinalitySelect
	_, err := f.db.From("block_finality").Select("last_validated_block_number", "", false).Eq("block_tag", blockTag).ExecuteTo(&blockFinality)
	if err != nil {
		return 0, err
	}

	if len(blockFinality) == 0 {
		return 0, nil
	}

	lastValidatedBlockNumber := blockFinality[0].LastValidatedBlockNumber

	return lastValidatedBlockNumber, nil
}

func (f *FinalityProcessor) getNextEventToValidate(blockNumber int64) (*database.PublicEventsSelect, error) {

	f.logger.Info("Getting next event to validate greater than block number: %v", blockNumber)
	var nextEvents []database.PublicEventsSelect
	_, err := f.db.From("events").Select("id,block_number, block_hash", "", false).Gt("block_number", strconv.FormatInt(blockNumber, 10)).Limit(1, "").
		Order("block_number", &postgrest.OrderOpts{Ascending: true}).
		ExecuteTo(&nextEvents)
	if err != nil {
		return nil, err
	}

	if len(nextEvents) == 0 {
		return nil, nil
	}

	return &nextEvents[0], nil

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
