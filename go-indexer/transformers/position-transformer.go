package transformers

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

var ZERO_ADDRESS = common.HexToAddress("0x0000000000000000000000000000000000000000")

type PositionTransformer struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
	logger *logger.Logger
}

func NewPositionTransformer(db *supa.Client) *PositionTransformer {
	ctx, cancel := context.WithCancel(context.Background())
	return &PositionTransformer{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
		logger: logger.NewLogger("Transformer:Position"),
	}
}

type ProcessPosition struct {
	ReceiverAddress string
	SenderAddress   string
	ContractAddress string
	AmountShares    string
	BlockNumber     uint64
}

func (p *PositionTransformer) Transfer(args ProcessPosition) ([]database.PositionInsert, []database.PositionUpdate, error) {

	// mint, handled by Deposit event
	if args.SenderAddress == ZERO_ADDRESS.Hex() {
		return nil, nil, nil
	}

	// burn, handled by Withdraw event
	if args.ReceiverAddress == ZERO_ADDRESS.Hex() {
		return nil, nil, nil
	}

	var receiverPosition *database.PublicPositionsSelect
	var senderPosition *database.PublicPositionsSelect
	var maxPositionIndexId int64

	senderPosition, err := p.GetMostRecentPosition(args.SenderAddress, args.ContractAddress)
	if err != nil {
		return nil, nil, err
	}

	receiverPosition, err = p.GetMostRecentPosition(args.ReceiverAddress, args.ContractAddress)
	if err != nil {
		return nil, nil, err
	}

	maxPositionIndexId, err = p.getMaxPositionIndexId(args.ContractAddress)
	if err != nil {
		p.logger.Error("Error getting max position index id: %v", err)
		return nil, nil, err
	}

	return p.ComputeTransfer(args, senderPosition, receiverPosition, maxPositionIndexId)

}

func (p *PositionTransformer) Withdraw(args ProcessPosition) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var senderPosition *database.PublicPositionsSelect
	var maxPositionIndexId int64

	senderPosition, err := p.GetMostRecentPosition(args.SenderAddress, args.ContractAddress)
	if err != nil {
		return nil, nil, err
	}

	maxPositionIndexId, err = p.getMaxPositionIndexId(args.ContractAddress)

	if err != nil {
		p.logger.Error("Error getting max position index id: %v", err)
		return nil, nil, err
	}

	return p.ComputeWithdraw(args, senderPosition, maxPositionIndexId)

}

func (p *PositionTransformer) Deposit(args ProcessPosition) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var senderPosition *database.PublicPositionsSelect
	var maxPositionIndexId int64

	senderPosition, err := p.GetMostRecentPosition(args.SenderAddress, args.ContractAddress)
	if err != nil {
		return nil, nil, err
	}

	maxPositionIndexId, err = p.getMaxPositionIndexId(args.ContractAddress)
	if err != nil {
		p.logger.Error("Error getting max position index id: %v", err)
		return nil, nil, err
	}
	return p.ComputeDeposit(args, senderPosition, maxPositionIndexId)

}

func (p *PositionTransformer) GetMostRecentPosition(address string, contractAddress string) (*database.PublicPositionsSelect, error) {
	if address == "" || address == ZERO_ADDRESS.Hex() || !common.IsHexAddress(address) {
		return nil, nil
	}

	data, _, err := p.db.From("positions").
		Select("*", "", false).
		Eq("owner_address", address).
		Eq("contract_address", contractAddress).
		Order("position_index_id", &postgrest.OrderOpts{Ascending: false}).
		Limit(1, "").
		Execute()

	if err != nil {
		return nil, nil
	}

	var positions []database.PublicPositionsSelect
	if err := json.Unmarshal(data, &positions); err != nil {
		p.logger.Error("Error unmarshaling position: %v", err)
		return nil, err
	}

	if len(positions) == 0 {
		// no position found
		return nil, nil
	}

	pos := positions[0]

	if pos.IsTerminated {
		return nil, nil
	}

	return &pos, nil
}

func (p *PositionTransformer) ComputeTransfer(args ProcessPosition, senderPosition *database.PublicPositionsSelect, receiverPosition *database.PublicPositionsSelect, maxPositionIndexId int64) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var updates []database.PositionUpdate
	var inserts []database.PositionInsert
	p.logger.Debug("ComputeTransfer: args: %v, senderPosition: %v, receiver position: %v, maxPositionIndexId: %v", args, senderPosition, receiverPosition, maxPositionIndexId)

	positionIndexId := maxPositionIndexId

	if receiverPosition == nil || receiverPosition.IsTerminated {
		// create a new position
		positionIndexId++
		inserts = append(inserts, database.ToPositionInsert(database.PublicPositionsInsert{
			PositionIndexId:     positionIndexId,
			OwnerAddress:        args.ReceiverAddress,
			ContractAddress:     args.ContractAddress,
			AmountShares:        args.AmountShares,
			PositionStartHeight: int64(args.BlockNumber),
		}))
	} else {

		// update receiver position
		insert, update, err := p.UpdatePosition(UpdatePositionInput{
			CurrentPosition:         receiverPosition,
			Address:                 args.ReceiverAddress,
			AmountShares:            args.AmountShares,
			BlockNumber:             args.BlockNumber,
			IsAddition:              true,
			WithdrawReceiverAddress: nil,
		}, &positionIndexId)
		if err != nil {
			p.logger.Error("Error updating position: %v", err)
			return inserts, updates, err
		}
		if update != nil {
			updates = append(updates, *update)
		}
		if insert != nil {
			inserts = append(inserts, *insert)
		}
	}

	// var isDeposit = args.SenderAddress == ZERO_ADDRESS.Hex()
	var isTransfer = args.SenderAddress != ZERO_ADDRESS.Hex() && args.ReceiverAddress != ZERO_ADDRESS.Hex()

	if isTransfer && senderPosition == nil {
		return inserts, updates, ErrPositionNotFound

	} else {
		// update sender position
		insert, update, err := p.UpdatePosition(UpdatePositionInput{
			CurrentPosition: senderPosition,
			Address:         args.SenderAddress,
			AmountShares:    args.AmountShares,
			BlockNumber:     args.BlockNumber,
			IsAddition:      false,
		}, &positionIndexId)
		if err != nil {
			p.logger.Error("error updating position: %v", err)
			return inserts, updates, err
		}
		if update != nil {
			updates = append(updates, *update)
		}
		if insert != nil {
			inserts = append(inserts, *insert)
		}
	}
	return inserts, updates, nil
}
func (p *PositionTransformer) ComputeWithdraw(args ProcessPosition, senderPosition *database.PublicPositionsSelect, maxPositionIndexId int64) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var updates []database.PositionUpdate
	var inserts []database.PositionInsert
	maxPositionIndexIdPtr := maxPositionIndexId

	p.logger.Debug("computing withdraw. args: %v", args)

	// update sender position
	insert, update, err := p.UpdatePosition(UpdatePositionInput{
		CurrentPosition:         senderPosition,
		Address:                 args.SenderAddress,
		AmountShares:            args.AmountShares,
		BlockNumber:             args.BlockNumber,
		IsAddition:              false,
		WithdrawReceiverAddress: &args.ReceiverAddress,
	}, &maxPositionIndexIdPtr)
	if err != nil {
		p.logger.Error("error updating position: %v", err)
		return inserts, updates, err
	}

	if update != nil {
		updates = append(updates, *update)
	}
	if insert != nil {
		inserts = append(inserts, *insert)
	}

	return inserts, updates, nil
}

func (p *PositionTransformer) ComputeDeposit(args ProcessPosition, existingPosition *database.PublicPositionsSelect, maxPositionIndexId int64) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var updates []database.PositionUpdate
	var inserts []database.PositionInsert
	maxPositionIndexIdPtr := maxPositionIndexId

	p.logger.Debug("computing withdraw. args: %v", args)

	if existingPosition == nil {
		maxPositionIndexIdPtr++
		inserts = append(inserts, database.ToPositionInsert(database.PublicPositionsInsert{
			PositionIndexId:     maxPositionIndexIdPtr,
			OwnerAddress:        args.SenderAddress,
			ContractAddress:     args.ContractAddress,
			AmountShares:        args.AmountShares,
			PositionStartHeight: int64(args.BlockNumber),
		}))

	} else {
		// update sender position
		insert, update, err := p.UpdatePosition(UpdatePositionInput{
			CurrentPosition: existingPosition,
			Address:         args.SenderAddress,
			AmountShares:    args.AmountShares,
			BlockNumber:     args.BlockNumber,
			IsAddition:      true,
		}, &maxPositionIndexIdPtr)

		if err != nil {
			p.logger.Error("error updating position: %v", err)
			return inserts, updates, err
		}
		if update != nil {
			updates = append(updates, *update)
		}
		if insert != nil {

			inserts = append(inserts, *insert)
		}

	}

	return inserts, updates, nil
}

type UpdatePositionInput struct {
	CurrentPosition         *database.PublicPositionsSelect
	Address                 string
	AmountShares            string
	BlockNumber             uint64
	IsAddition              bool
	WithdrawReceiverAddress *string
	PositionIndexId         int64
}

func (p *PositionTransformer) UpdatePosition(
	input UpdatePositionInput,
	maxPositionIndexId *int64,
) (*database.PositionInsert, *database.PositionUpdate, error) {

	if input.CurrentPosition == nil {
		return nil, nil, nil
	}

	p.logger.Debug("updating position. currentPosition: %v, address: %v, amountShares: %v, blockNumber: %v, isAddition: %v, withdrawReceiverAddress: %v", *input.CurrentPosition, input.Address, input.AmountShares, input.BlockNumber, input.IsAddition, input.WithdrawReceiverAddress)

	newAmountShares, err := computeNewAmountShares(input.CurrentPosition, input.AmountShares, input.IsAddition)
	if err != nil {
		p.logger.Error("error computing new amount shares: %v", err)
		return nil, nil, err
	}

	endHeight := int64(input.BlockNumber - 1)

	// Check if the position should be terminated (either zero balance)
	var isTerminated = newAmountShares == "0"

	p.logger.Debug("isTerminated: %v, newAmountShares: %v", isTerminated, newAmountShares)

	var insert *database.PositionInsert
	var update database.PositionUpdate

	if !isTerminated {
		p.logger.Debug("is not terminated. inserting position")
		*maxPositionIndexId++

		newInsert := database.ToPositionInsert(database.PublicPositionsInsert{
			OwnerAddress:        input.Address,
			ContractAddress:     input.CurrentPosition.ContractAddress,
			AmountShares:        newAmountShares,
			PositionStartHeight: int64(input.BlockNumber),
			PositionIndexId:     *maxPositionIndexId,
		})
		insert = &newInsert
	}

	p.logger.Debug("insert value: %v", insert)

	update = database.ToPositionUpdate(database.PublicPositionsUpdate{
		Id:                      &input.CurrentPosition.Id,
		IsTerminated:            &isTerminated,
		PositionEndHeight:       &endHeight,
		WithdrawReceiverAddress: input.WithdrawReceiverAddress,
	})

	return insert, &update, nil
}

func computeNewAmountShares(currentPosition *database.PublicPositionsSelect, newAmountShares string, isAddition bool) (string, error) {
	if currentPosition == nil {
		return newAmountShares, nil
	}

	currentBigInt := new(big.Int)
	if _, ok := currentBigInt.SetString(currentPosition.AmountShares, 10); !ok {
		// If we can't parse the current position's amount, return "0" to prevent invalid operations
		return "0", errors.New("failed to parse current position amount")
	}

	newBigInt := new(big.Int)
	if _, ok := newBigInt.SetString(newAmountShares, 10); !ok {
		// If we can't parse the new amount, return the current amount unchanged
		return currentPosition.AmountShares, errors.New("failed to parse new amount")
	}

	if isAddition {
		newBigInt.Add(currentBigInt, newBigInt)
	} else {
		newBigInt.Sub(currentBigInt, newBigInt)
		// If subtraction results in negative value, clamp to zero
		if newBigInt.Sign() < 0 {
			newBigInt.SetInt64(0)
		}
	}
	return newBigInt.String(), nil
}

func (p *PositionTransformer) getMaxPositionIndexId(contractAddress string) (int64, error) {

	var maxPositionIndexId int64

	data, _, err := p.db.From("positions").
		Select("position_index_id", "", false).
		Eq("contract_address", contractAddress).
		Order("position_index_id", &postgrest.OrderOpts{Ascending: false}).
		Limit(1, "").
		Single().
		Execute()

	if err != nil {

		// Check if this is the "no rows returned" error
		var errStr = err.Error()
		if strings.Contains(errStr, "no rows") || strings.Contains(errStr, "PGRST116") {
			maxPositionIndexId = 0
		} else {
			p.logger.Error("Error getting max position index id: %v", err)
			return 0, err
		}
	} else {

		var posId database.PublicPositionsSelect
		if err := json.Unmarshal(data, &posId); err != nil {
			p.logger.Error("Error unmarshaling max position index id: %v", err)
			return 0, err
		}
		maxPositionIndexId = posId.PositionIndexId

	}

	return maxPositionIndexId, nil
}

func (p *PositionTransformer) CleanupFromBlock() []string {

	return []string{
		"DELETE FROM positions WHERE position_start_height > $1;",
		"UPDATE positions SET is_terminated = false, position_end_height = null, withdraw_receiver_address = null where position_end_height > $1;",
	}
}
