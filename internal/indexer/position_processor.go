package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/internal/database"
)

var ZERO_ADDRESS = common.HexToAddress("0x0000000000000000000000000000000000000000")

// PositionEvent represents an event that needs to be processed for position updates
type PositionEvent struct {
	EventName string
	EventData map[string]interface{}
	Log       types.Log
}

// PositionProcessor handles position updates from events
type PositionProcessor struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPositionProcessor creates a new position processor
func NewPositionProcessor(db *supa.Client) *PositionProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &PositionProcessor{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (p *PositionProcessor) Start(eventChan <-chan PositionEvent) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					// Channel is closed, exit gracefully
					return
				}
				// Get current position if it exists
				var senderAddress string
				var receiverAddress string
				var contractAddress = event.Log.Address.Hex()
				var receiverPosition *database.PublicPositionsSelect
				var senderPosition *database.PublicPositionsSelect

				// Determine the ethereum address based on event type
				switch event.EventName {
				case "Transfer":
					if to, ok := event.EventData["to"].(common.Address); ok {
						receiverAddress = to.Hex()
					}
					if from, ok := event.EventData["from"].(common.Address); ok {
						senderAddress = from.Hex()
					}

				case "WithdrawRequested":
					if from, ok := event.EventData["owner"].(common.Address); ok {
						senderAddress = from.Hex()
					}
				}

				if senderAddress != "" && senderAddress != ZERO_ADDRESS.Hex() {
					data, _, err := p.db.From("positions").
						Select("*", "", false).
						Eq("ethereum_address", senderAddress).
						Eq("contract_address", contractAddress).
						Order("id", &postgrest.OrderOpts{Ascending: false}).
						Limit(1, "").
						Single().
						Execute()

					if err == nil {
						var pos database.PublicPositionsSelect
						if err := json.Unmarshal(data, &pos); err != nil {
							log.Printf("Error unmarshaling sender position: %v", err)
							continue
						}
						senderPosition = &pos
					}
				}

				if receiverAddress != "" && receiverAddress != ZERO_ADDRESS.Hex() {
					data, _, err := p.db.From("positions").
						Select("*", "", false).
						Eq("ethereum_address", receiverAddress).
						Eq("contract_address", contractAddress).
						Order("id", &postgrest.OrderOpts{Ascending: false}).
						Limit(1, "").
						Single().
						Execute()

					if err == nil {
						var pos database.PublicPositionsSelect
						if err := json.Unmarshal(data, &pos); err != nil {
							log.Printf("Error unmarshaling current position: %v", err)
							continue
						}
						receiverPosition = &pos
					}
				}

				// Process the event
				inserts, updates, err := p.processPositionEvent(event, receiverPosition, senderPosition)
				if err != nil {
					log.Printf("Error processing position event: %v", err)
					continue
				}

				for _, update := range updates {
					_, _, err = p.db.From("positions").Update(update, "", "").
						Eq("id", fmt.Sprintf("%v", *update.Id)).
						Execute()
					if err != nil {
						log.Printf("Error updating position: %v", err)
						continue
					}
				}
				for _, insert := range inserts {
					_, _, err = p.db.From("positions").Insert(insert, false, "", "", "").Execute()
					if err != nil {
						log.Printf("Error inserting new position: %v", err)
						continue
					}
				}

			case <-p.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *PositionProcessor) processPositionEvent(event PositionEvent, receiverPosition *database.PublicPositionsSelect, senderPosition *database.PublicPositionsSelect) ([]database.PublicPositionsInsert, []database.PublicPositionsUpdate, error) {

	var updates []database.PublicPositionsUpdate
	var inserts []database.PublicPositionsInsert

	switch event.EventName {
	case "Transfer":
		var receiverAddress string
		var senderAddress string
		if receiver, ok := event.EventData["to"].(common.Address); ok {
			receiverAddress = receiver.Hex()
		}
		if sender, ok := event.EventData["from"].(common.Address); ok {
			senderAddress = sender.Hex()
		}
		// 1. update receiver. if receiver address is 0x0000000000000000000000000000000000000000, do not process reciever position change
		// 2. update sender. if sender address 0x0000000000000000000000000000000000000000, do not process sender position change

		var amount_shares string
		var isWithdraw = receiverAddress == ZERO_ADDRESS.Hex()
		if value, ok := event.EventData["value"].(*big.Int); ok {
			amount_shares = value.String()
		}

		if isWithdraw {
			// exit early
			return inserts, updates, nil
		}
		if receiverAddress == ZERO_ADDRESS.Hex() {
			// do nothing
		} else if receiverPosition == nil {
			// create a new position
			inserts = append(inserts, database.PublicPositionsInsert{
				EthereumAddress:     receiverAddress,
				ContractAddress:     event.Log.Address.Hex(),
				AmountShares:        amount_shares,
				PositionStartHeight: int64(event.Log.BlockNumber),
				PositionEndHeight:   nil,
				IsTerminated:        nil, // this is only incrementing, can't be terminated
				NeutronAddress:      nil,
			})
		} else {
			// update receiver position
			insert, update := updatePosition(receiverPosition, receiverAddress, amount_shares, event.Log.BlockNumber, true)
			if update != nil {
				updates = append(updates, *update)
			}
			if insert != nil {
				inserts = append(inserts, *insert)
			}
		}

		if senderAddress == ZERO_ADDRESS.Hex() {
			// do nothing
		} else if senderPosition == nil {
			// do nothing
		} else {
			// update sender position
			insert, update := updatePosition(senderPosition, senderAddress, amount_shares, event.Log.BlockNumber, false)
			if update != nil {
				updates = append(updates, *update)
			}
			if insert != nil {
				inserts = append(inserts, *insert)
			}
		}

	case "WithdrawRequested":
		// update withdrawer position
		var amount_shares string
		var neutronAddress = event.EventData["receiver"].(string)

		if value, ok := event.EventData["shares"].(*big.Int); ok {
			amount_shares = value.String()
		}
		var senderAddress string
		if sender, ok := event.EventData["owner"].(common.Address); ok {
			senderAddress = sender.Hex()
		}

		insert, update := updatePosition(senderPosition, senderAddress, amount_shares, event.Log.BlockNumber, false)
		if update != nil {
			update.NeutronAddress = &neutronAddress
			updates = append(updates, *update)
		}
		if insert != nil {
			inserts = append(inserts, *insert)
		}

	default:
		// exit early
		return inserts, updates, nil
	}

	return inserts, updates, nil
}

func updatePosition(
	currentPosition *database.PublicPositionsSelect,
	address string,
	amountShares string,
	blockNumber uint64,
	isAddition bool,
) (*database.PublicPositionsInsert, *database.PublicPositionsUpdate) {
	if currentPosition == nil {
		return nil, nil
	}
	newAmountShares := computeNewAmountShares(currentPosition, amountShares, isAddition)
	endHeight := int64(blockNumber - 1)
	var isTerminated = newAmountShares == "0" // will only be relevant if not adding

	var insert *database.PublicPositionsInsert
	var update *database.PublicPositionsUpdate

	if !isTerminated {
		insert = &database.PublicPositionsInsert{
			EthereumAddress:     address,
			ContractAddress:     currentPosition.ContractAddress,
			AmountShares:        newAmountShares,
			PositionStartHeight: int64(blockNumber),
		}
	}

	update = &database.PublicPositionsUpdate{
		Id: &currentPosition.Id,

		// new values
		IsTerminated:      &isTerminated,
		PositionEndHeight: &endHeight,

		// preserve
		ContractAddress:     &currentPosition.ContractAddress,
		EthereumAddress:     &currentPosition.EthereumAddress,
		AmountShares:        &currentPosition.AmountShares,
		PositionStartHeight: &currentPosition.PositionStartHeight,
		CreatedAt:           &currentPosition.CreatedAt,
	}

	return insert, update
}

func computeNewAmountShares(currentPosition *database.PublicPositionsSelect, newAmountShares string, isAddition bool) string {
	if currentPosition == nil {
		return newAmountShares
	}

	currentBigInt := new(big.Int)
	currentBigInt.SetString(currentPosition.AmountShares, 10)
	newBigInt := new(big.Int)
	newBigInt.SetString(newAmountShares, 10)

	if isAddition {
		newBigInt.Add(currentBigInt, newBigInt)
	} else {
		newBigInt.Sub(currentBigInt, newBigInt)
	}
	return newBigInt.String()
}

func (p *PositionProcessor) Stop() {
	p.cancel()
	p.wg.Wait()
}
