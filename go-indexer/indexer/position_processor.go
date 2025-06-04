package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
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
	log.Println("Starting position processor...")
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
				var maxPositionIndexId int64

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
						Order("position_index_id", &postgrest.OrderOpts{Ascending: false}).
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
						continue
					}
				} else {

					var posId database.PublicPositionsSelect
					if err := json.Unmarshal(data, &posId); err != nil {
						log.Printf("Error unmarshaling max position index id: %v", err)
						continue
					}
					maxPositionIndexId = posId.PositionIndexId

				}

				// Process the event
				inserts, updates, err := p.processPositionEvent(event, senderPosition, receiverPosition, maxPositionIndexId)
				if err != nil {
					log.Printf("Error processing position event: %v", err)
					continue
				}

				for _, update := range updates {
					_, _, err = p.db.From("positions").Update(update, "", "").
						Eq("id", update.Id).
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

func (p *PositionProcessor) processPositionEvent(event PositionEvent, senderPos *database.PublicPositionsSelect, receiverPos *database.PublicPositionsSelect, maxPositionIndexId int64) ([]database.PublicPositionsInsert, []database.PublicPositionsUpdate, error) {

	var updates []database.PublicPositionsUpdate
	var inserts []database.PublicPositionsInsert
	positionIndexId := maxPositionIndexId

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
			// exit early.  This is will written by the withdraw event, which includes the same info + the neutron address
			return inserts, updates, nil
		}
		if receiverAddress == ZERO_ADDRESS.Hex() {
			// do nothing
		} else if receiverPos == nil {

			// create a new position
			positionIndexId++
			inserts = append(inserts, database.PublicPositionsInsert{
				PositionIndexId:         positionIndexId,
				OwnerAddress:            receiverAddress,
				ContractAddress:         event.Log.Address.Hex(),
				AmountShares:            amount_shares,
				PositionStartHeight:     int64(event.Log.BlockNumber),
				PositionEndHeight:       nil,
				IsTerminated:            nil, // this is only incrementing, can't be terminated
				WithdrawRecieverAddress: nil,
			})

		} else {
			// update receiver position
			insert, update := updatePosition(receiverPos, receiverAddress, amount_shares, event.Log.BlockNumber, true, nil, &positionIndexId)
			if update != nil {
				updates = append(updates, *update)
			}
			if insert != nil {
				inserts = append(inserts, *insert)
			}
		}

		if senderAddress == ZERO_ADDRESS.Hex() {
			// do nothing
		} else if senderPos == nil {
			// do nothing
		} else {
			// update sender position
			insert, update := updatePosition(senderPos, senderAddress, amount_shares, event.Log.BlockNumber, false, nil, &positionIndexId)
			if update != nil {
				updates = append(updates, *update)
			}
			if insert != nil {
				inserts = append(inserts, *insert)
			}
		}
		log.Printf("%v Transfer: from: %v, to: %v, value: %v", event.Log.Address.Hex(), senderAddress, receiverAddress, amount_shares)

	case "WithdrawRequested":
		// update withdrawer position
		var amount_shares string
		neutronAddress, ok := event.EventData["receiver"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("invalid receiver address type in WithdrawRequested event")
		}

		if value, ok := event.EventData["shares"].(*big.Int); ok {
			amount_shares = value.String()
		}
		var senderAddress string
		if sender, ok := event.EventData["owner"].(common.Address); ok {
			senderAddress = sender.Hex()
		}

		insert, update := updatePosition(senderPos, senderAddress, amount_shares, event.Log.BlockNumber, false, &neutronAddress, &positionIndexId)
		if update != nil {
			updates = append(updates, *update)
		}
		if insert != nil {
			inserts = append(inserts, *insert)
		}
		log.Printf("%v WithdrawRequested: from: %v, to: %v, value: %v", event.Log.Address.Hex(), senderAddress, neutronAddress, amount_shares)

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
	neutronAddress *string,
	maxPositionIndexId *int64,
) (*database.PublicPositionsInsert, *database.PublicPositionsUpdate) {
	if currentPosition == nil {
		return nil, nil
	}
	newAmountShares := computeNewAmountShares(currentPosition, amountShares, isAddition)
	endHeight := int64(blockNumber - 1)

	// Check if the position should be terminated (either zero balance)
	var isTerminated = newAmountShares == "0"

	var insert *database.PublicPositionsInsert
	var update *database.PublicPositionsUpdate

	if !isTerminated {
		*maxPositionIndexId++
		insert = &database.PublicPositionsInsert{
			OwnerAddress:        address,
			ContractAddress:     currentPosition.ContractAddress,
			AmountShares:        newAmountShares,
			PositionStartHeight: int64(blockNumber),
			PositionIndexId:     *maxPositionIndexId,
		}
	}

	update = &database.PublicPositionsUpdate{
		Id: currentPosition.Id,

		// new values
		IsTerminated:            &isTerminated,
		PositionEndHeight:       &endHeight,
		WithdrawRecieverAddress: neutronAddress,
	}

	return insert, update
}

func computeNewAmountShares(currentPosition *database.PublicPositionsSelect, newAmountShares string, isAddition bool) string {
	if currentPosition == nil {
		return newAmountShares
	}

	currentBigInt := new(big.Int)
	if _, ok := currentBigInt.SetString(currentPosition.AmountShares, 10); !ok {
		// If we can't parse the current position's amount, return "0" to prevent invalid operations
		return "0"
	}

	newBigInt := new(big.Int)
	if _, ok := newBigInt.SetString(newAmountShares, 10); !ok {
		// If we can't parse the new amount, return the current amount unchanged
		return currentPosition.AmountShares
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
	return newBigInt.String()
}

func (p *PositionProcessor) Stop() {
	p.cancel()
	p.wg.Wait()
}
