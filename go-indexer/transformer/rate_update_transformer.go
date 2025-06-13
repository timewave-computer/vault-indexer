package transformer

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

type RateUpdateTransformer struct {
	db        *supa.Client
	ctx       context.Context
	ethClient *ethclient.Client
	cancel    context.CancelFunc
}

func NewRateUpdateTransformer(db *supa.Client, ethClient *ethclient.Client) *RateUpdateTransformer {
	ctx, cancel := context.WithCancel(context.Background())

	return &RateUpdateTransformer{
		db:        db,
		ctx:       ctx,
		cancel:    cancel,
		ethClient: ethClient,
	}
}

type ProcessRateUpdate struct {
	ContractAddress string
	Rate            string
	BlockNumber     int64
}

func (w *RateUpdateTransformer) Transform(args ProcessRateUpdate) (database.PublicRateUpdatesInsert, error) {
	// Create the insert struct

	blockHeader, err := w.ethClient.HeaderByNumber(w.ctx, big.NewInt(args.BlockNumber))
	if err != nil {
		return database.PublicRateUpdatesInsert{}, err
	}

	blockTimestamp := blockHeader.Time

	utcTime := time.Unix(int64(blockTimestamp), 0).UTC().Format(time.RFC3339)

	rateUpdate := database.PublicRateUpdatesInsert{
		BlockNumber:     args.BlockNumber,
		ContractAddress: args.ContractAddress,
		Rate:            args.Rate,
		BlockTimestamp:  utcTime,
	}

	return rateUpdate, nil
}
