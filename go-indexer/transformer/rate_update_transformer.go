package transformer

import (
	"context"

	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

type RateUpdateTransformer struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRateUpdateTransformer(db *supa.Client) *RateUpdateTransformer {
	ctx, cancel := context.WithCancel(context.Background())
	return &RateUpdateTransformer{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

type ProcessRateUpdate struct {
	ContractAddress string
	Rate            string
	BlockNumber     int64
}

func (w *RateUpdateTransformer) Transform(args ProcessRateUpdate) (database.PublicRateUpdatesInsert, error) {
	// Create the insert struct
	rateUpdate := database.PublicRateUpdatesInsert{
		BlockNumber:     args.BlockNumber,
		ContractAddress: args.ContractAddress,
		Rate:            args.Rate,
	}

	return rateUpdate, nil
}
