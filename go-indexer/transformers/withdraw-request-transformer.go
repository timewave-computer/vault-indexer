package transformers

import (
	"context"

	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

type WithdrawRequestTransformer struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWithdrawRequestTransformer(db *supa.Client) *WithdrawRequestTransformer {
	ctx, cancel := context.WithCancel(context.Background())
	return &WithdrawRequestTransformer{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

type ProcessWithdrawRequest struct {
	OwnerAddress    string
	ReceiverAddress string
	Amount          string
	WithdrawId      int64
	BlockNumber     uint64
	ContractAddress string
}

func (w *WithdrawRequestTransformer) Transform(args ProcessWithdrawRequest) (database.PublicWithdrawRequestsInsert, error) {
	// Create the withdraw request insert struct
	withdrawRequest := database.PublicWithdrawRequestsInsert{
		Amount:          args.Amount,
		BlockNumber:     int64(args.BlockNumber),
		ContractAddress: args.ContractAddress,
		OwnerAddress:    args.OwnerAddress,
		ReceiverAddress: args.ReceiverAddress,
		WithdrawId:      args.WithdrawId,
	}

	return withdrawRequest, nil
}
