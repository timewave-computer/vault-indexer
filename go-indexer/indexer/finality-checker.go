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

type FinalityChecker struct {
	logger    *logger.Logger
	ethClient *ethclient.Client
	db        *supa.Client
	ctx       context.Context
	cancel    context.CancelFunc
	once      sync.Once
	wg        sync.WaitGroup
}

func NewFinalityChecker(ethClient *ethclient.Client, db *supa.Client) *FinalityChecker {
	logger := logger.NewLogger("FinalityChecker")
	ctx, cancel := context.WithCancel(context.Background())

	return &FinalityChecker{
		logger:    logger,
		ethClient: ethClient,
		db:        db,
		ctx:       ctx,
		cancel:    cancel,
		wg:        sync.WaitGroup{},
	}
}

func (f *FinalityChecker) Start() error {

	errors := make(chan error, 10)

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case err := <-errors:
				f.logger.Error("Error in finality checker: %v", err)
				f.Stop()
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

				f.logger.Info("Checking finality")
				// Get the safe block (use SafeBlockNumber, not FinalizedBlockNumber)
				canonicalSafeBlock, err := f.ethClient.HeaderByNumber(context.Background(), big.NewInt(int64(rpc.SafeBlockNumber)))
				if err != nil {
					f.logger.Error("Error getting last safe block: %v", err)
					errors <- err
					return
				}

				// Get the finalized block
				canonicalFinalizedBlock, err := f.ethClient.HeaderByNumber(context.Background(), big.NewInt(int64(rpc.FinalizedBlockNumber)))
				if err != nil {
					f.logger.Error("Error getting last finalized block: %v", err)
					errors <- err
					return
				}

				// Use the blocks for your logic
				f.logger.Info("Safe block number: %d", canonicalSafeBlock.Number)
				f.logger.Info("Finalized block number: %d", canonicalFinalizedBlock.Number)

				// Fix the Supabase query - ExecuteTo returns only error and result
				var blockFinality database.PublicBlockFinalitySelect
				data, err := f.db.From("block_finality").Select("*", "", false).Single().ExecuteTo(&blockFinality)
				if err != nil {
					f.logger.Error("Error getting block finality: %v", err)
					errors <- err
					return
				}

				// Use the retrieved data
				_ = data
				f.logger.Info("Block finality data retrieved: %+v", blockFinality)

				time.Sleep(15 * time.Second)

			}
		}
	}()

	return nil
}

func (f *FinalityChecker) Stop() {
	f.once.Do(func() {
		f.logger.Info("Stopping finality checker...")
		f.cancel()
	})
}
