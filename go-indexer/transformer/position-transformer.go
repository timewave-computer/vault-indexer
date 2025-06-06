package transformer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

var ZERO_ADDRESS = common.HexToAddress("0x0000000000000000000000000000000000000000")

type PositionTransformer struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewPositionTransformer(db *supa.Client) *PositionTransformer {
	ctx, cancel := context.WithCancel(context.Background())
	return &PositionTransformer{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

type ProcessPosition struct {
	ReceiverAddress string
	SenderAddress   string
	ContractAddress string
	AmountShares    string
	BlockNumber     uint64
}

func (p *PositionTransformer) ProcessPositionTransformation(args ProcessPosition, isWithdraw bool) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var receiverPosition *database.PublicPositionsSelect
	var senderPosition *database.PublicPositionsSelect
	var maxPositionIndexId int64

	if args.SenderAddress != "" && args.SenderAddress != ZERO_ADDRESS.Hex() && common.IsHexAddress(args.SenderAddress) {
		data, _, err := p.db.From("positions").
			Select("*", "", false).
			Eq("owner_address", args.SenderAddress).
			Eq("contract_address", args.ContractAddress).
			Order("id", &postgrest.OrderOpts{Ascending: false}).
			Limit(1, "").
			Single().
			Execute()

		if err == nil {
			var pos database.PublicPositionsSelect
			if err := json.Unmarshal(data, &pos); err != nil {
				log.Printf("Error unmarshaling sender position: %v", err)
				return nil, nil, err
			}
			senderPosition = &pos
		}
	}

	if args.ReceiverAddress != "" && args.ReceiverAddress != ZERO_ADDRESS.Hex() && common.IsHexAddress(args.ReceiverAddress) {
		data, _, err := p.db.From("positions").
			Select("*", "", false).
			Eq("owner_address", args.ReceiverAddress).
			Eq("contract_address", args.ContractAddress).
			Order("position_index_id", &postgrest.OrderOpts{Ascending: false}).
			Limit(1, "").
			Single().
			Execute()

		if err == nil {
			var pos database.PublicPositionsSelect
			if err := json.Unmarshal(data, &pos); err != nil {
				log.Printf("Error unmarshaling current position: %v", err)
				return nil, nil, err
			}
			receiverPosition = &pos
		}
	}

	maxPositionIndexId, err := p.getMaxPositionIndexId(args.ContractAddress)
	if err != nil {
		log.Printf("Error getting max position index id: %v", err)
		return nil, nil, err
	}

	if isWithdraw {
		return p.computeWithdraw(args, senderPosition, receiverPosition, maxPositionIndexId)
	} else {
		return p.computeTransfer(args, senderPosition, receiverPosition, maxPositionIndexId)
	}
}

func (p *PositionTransformer) computeTransfer(args ProcessPosition, senderPosition *database.PublicPositionsSelect, receiverPosition *database.PublicPositionsSelect, maxPositionIndexId int64) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var updates []database.PositionUpdate
	var inserts []database.PositionInsert
	positionIndexId := maxPositionIndexId
	log.Printf("maxPositionIndexId: %v", maxPositionIndexId)

	var isTransferWithdraw = args.ReceiverAddress == ZERO_ADDRESS.Hex()

	if isTransferWithdraw {
		// exit early.  This is will written by the withdraw event, which includes the same info + the neutron address
		return inserts, updates, nil
	}

	if receiverPosition == nil || *receiverPosition.IsTerminated {
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
		insert, update := updatePosition(UpdatePositionInput{
			CurrentPosition: receiverPosition,
			Address:         args.ReceiverAddress,
			AmountShares:    args.AmountShares,
			BlockNumber:     args.BlockNumber,
			IsAddition:      true,
		}, &positionIndexId)
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
		if senderPosition == nil {
			return inserts, updates, ErrPositionNotFound
		}
	} else {
		// update sender position
		insert, update := updatePosition(UpdatePositionInput{
			CurrentPosition: senderPosition,
			Address:         args.SenderAddress,
			AmountShares:    args.AmountShares,
			BlockNumber:     args.BlockNumber,
			IsAddition:      false,
		}, &positionIndexId)
		if update != nil {
			updates = append(updates, *update)
		}
		if insert != nil {
			inserts = append(inserts, *insert)
		}
	}
	return inserts, updates, nil
}
func (p *PositionTransformer) computeWithdraw(args ProcessPosition, senderPosition *database.PublicPositionsSelect, receiverPosition *database.PublicPositionsSelect, maxPositionIndexId int64) ([]database.PositionInsert, []database.PositionUpdate, error) {
	var updates []database.PositionUpdate
	var inserts []database.PositionInsert
	positionIndexId := maxPositionIndexId

	log.Printf("computing withdraw. args: %v", args)

	// update sender position
	insert, update := updatePosition(UpdatePositionInput{
		CurrentPosition:         senderPosition,
		Address:                 args.SenderAddress,
		AmountShares:            args.AmountShares,
		BlockNumber:             args.BlockNumber,
		IsAddition:              false,
		WithdrawReceiverAddress: &args.ReceiverAddress,
	}, &positionIndexId)

	if update != nil {
		log.Printf("got update: %v", *update)
		updates = append(updates, *update)
	}
	if insert != nil {
		log.Printf("got insert: %v", *insert)
		inserts = append(inserts, *insert)
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

func updatePosition(
	input UpdatePositionInput,
	positionIndexId *int64,
) (*database.PositionInsert, *database.PositionUpdate) {

	if input.CurrentPosition == nil {
		return nil, nil
	}

	log.Printf("updating position. currentPosition: %v, address: %v, amountShares: %v, blockNumber: %v, isAddition: %v, withdrawReceiverAddress: %v", *input.CurrentPosition, input.Address, input.AmountShares, input.BlockNumber, input.IsAddition, input.WithdrawReceiverAddress)

	newAmountShares, err := computeNewAmountShares(input.CurrentPosition, input.AmountShares, input.IsAddition)
	if err != nil {
		log.Printf("error computing new amount shares: %v", err)
		return nil, nil
	}

	endHeight := int64(input.BlockNumber - 1)

	// Check if the position should be terminated (either zero balance)
	var isTerminated = newAmountShares == "0"

	log.Printf("isTerminated: %v, newAmountShares: %v", isTerminated, newAmountShares)

	var insert *database.PositionInsert
	var update database.PositionUpdate

	if !isTerminated {
		log.Printf("is not terminated. inserting position")
		*positionIndexId++

		newInsert := database.ToPositionInsert(database.PublicPositionsInsert{
			OwnerAddress:        input.Address,
			ContractAddress:     input.CurrentPosition.ContractAddress,
			AmountShares:        newAmountShares,
			PositionStartHeight: int64(input.BlockNumber),
			PositionIndexId:     *positionIndexId,
		})
		insert = &newInsert
	}

	log.Printf("insert value: %v", insert)

	var receiverAddress string
	if input.WithdrawReceiverAddress != nil {
		receiverAddress = *input.WithdrawReceiverAddress
	}

	update = database.ToPositionUpdate(database.PublicPositionsUpdate{
		Id:                      &input.CurrentPosition.Id,
		IsTerminated:            &isTerminated,
		PositionEndHeight:       &endHeight,
		WithdrawReceiverAddress: &receiverAddress,
	})

	return insert, &update
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
			log.Printf("Error getting max position index id: %v", err)
			return 0, err
		}
	} else {

		var posId database.PublicPositionsSelect
		if err := json.Unmarshal(data, &posId); err != nil {
			log.Printf("Error unmarshaling max position index id: %v", err)
			return 0, err
		}
		maxPositionIndexId = posId.PositionIndexId

	}

	return maxPositionIndexId, nil
}
