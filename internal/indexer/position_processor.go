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
	supa "github.com/supabase-community/supabase-go"
)

// PositionEvent represents an event that needs to be processed for position updates
type PositionEvent struct {
	EventName string
	EventData map[string]interface{}
	Log       types.Log
}

// Position represents a position record in the database
type Position struct {
	ID                  int64       `json:"id"`
	Amount              json.Number `json:"amount"`
	PositionEndHeight   *int64      `json:"position_end_height"`
	IsTerminated        bool        `json:"is_terminated"`
	NeutronAddress      *string     `json:"neutron_address"`
	PositionStartHeight int64       `json:"position_start_height"`
	EthereumAddress     string      `json:"ethereum_address"`
	ContractAddress     string      `json:"contract_address"`
}

// PositionUpdate represents a position record to be upserted
type PositionUpdate struct {
	Id                  *int64
	EthereumAddress     string
	ContractAddress     string
	Amount              string
	PositionStartHeight uint64
	PositionEndHeight   *uint64
	IsTerminated        bool
	NeutronAddress      *string
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
			case event := <-eventChan:
				// Get current position if it exists
				var currentPosition *Position
				var ethereumAddress string

				// Determine the ethereum address based on event type
				switch event.EventName {
				case "Deposit":
					if owner, ok := event.EventData["sender"].(common.Address); ok {
						ethereumAddress = owner.Hex()
					}
				case "Transfer":
					if from, ok := event.EventData["from"].(common.Address); ok {
						ethereumAddress = from.Hex()
					}
				case "Withdraw":
					if owner, ok := event.EventData["sender"].(common.Address); ok {
						ethereumAddress = owner.Hex()
					}
				}

				if ethereumAddress == "" {
					log.Printf("Error: could not determine ethereum address from event data")
					continue
				}

				data, _, err := p.db.From("positions").
					Select("id,amount,position_end_height", "", false).
					Eq("ethereum_address", ethereumAddress).
					Eq("contract_address", event.Log.Address.Hex()).
					Is("position_end_height", "null").
					Single().
					Execute()

				if err == nil {
					var pos Position
					if err := json.Unmarshal(data, &pos); err != nil {
						log.Printf("Error unmarshaling current position: %v", err)
						continue
					}
					currentPosition = &pos
				}

				// Process the event
				updates, err := p.processPositionEvent(event, currentPosition)
				if err != nil {
					log.Printf("Error processing position event: %v", err)
					continue
				}

				// Apply updates to database
				for _, update := range updates {
					if update.PositionEndHeight != nil {
						// Update existing position
						_, _, err = p.db.From("positions").
							Update(map[string]interface{}{
								"position_end_height": update.PositionEndHeight,
								"is_terminated":       update.IsTerminated,
							}, "", "").
							Eq("ethereum_address", update.EthereumAddress).
							Eq("contract_address", update.ContractAddress).
							Is("position_end_height", "null").
							Execute()
						if err != nil {
							log.Printf("Error updating position: %v", err)
							continue
						}
					}

					if update.Amount != "0" {
						// Insert new position
						_, _, err = p.db.From("positions").Insert(map[string]interface{}{
							"ethereum_address":      update.EthereumAddress,
							"contract_address":      update.ContractAddress,
							"amount":                update.Amount,
							"position_start_height": update.PositionStartHeight,
							"position_end_height":   update.PositionEndHeight,
							"is_terminated":         update.IsTerminated,
							"neutron_address":       update.NeutronAddress,
						}, false, "", "", "").Execute()
						if err != nil {
							log.Printf("Error inserting new position: %v", err)
							continue
						}
					}
				}

			case <-p.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *PositionProcessor) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *PositionProcessor) processPositionEvent(event PositionEvent, currentPosition *Position) ([]PositionUpdate, error) {
	var ethereumAddress string
	var amount string

	// Handle different event types
	switch event.EventName {
	case "Deposit":
		if owner, ok := event.EventData["sender"].(common.Address); ok {
			ethereumAddress = owner.Hex()
		}
		if assets, ok := event.EventData["assets"].(*big.Int); ok {
			amount = assets.String()
		}

	case "Transfer":
		if to, ok := event.EventData["from"].(common.Address); ok {
			ethereumAddress = to.Hex()
		}
		if value, ok := event.EventData["value"].(*big.Int); ok {
			amount = value.String()
		}

		// TODO: update 2 positions (from + to)

	case "Withdraw":
		if owner, ok := event.EventData["sender"].(common.Address); ok {
			ethereumAddress = owner.Hex()
		}
		if assets, ok := event.EventData["assets"].(*big.Int); ok {
			// For withdrawals, we'll store the negative value as a string
			negAssets := new(big.Int).Neg(assets)
			amount = negAssets.String()
		}

	default:
		return nil, nil
	}

	if ethereumAddress == "" {
		return nil, fmt.Errorf("could not determine account address from event data")
	}

	// Calculate new amount
	newAmount := amount
	if currentPosition != nil {
		// Add the current amount to the new amount using big.Int
		currentBigInt := new(big.Int)
		currentBigInt.SetString(currentPosition.Amount.String(), 10)
		newBigInt := new(big.Int)
		newBigInt.SetString(amount, 10)
		newBigInt.Add(currentBigInt, newBigInt)
		newAmount = newBigInt.String()
	}

	var updates []PositionUpdate

	// Close current position if it exists
	if currentPosition != nil {
		endHeight := uint64(event.Log.BlockNumber - 1)
		updates = append(updates, PositionUpdate{
			Id:                  &currentPosition.ID,
			EthereumAddress:     ethereumAddress,
			ContractAddress:     event.Log.Address.Hex(),
			Amount:              currentPosition.Amount.String(),
			PositionStartHeight: uint64(currentPosition.PositionStartHeight),
			PositionEndHeight:   &endHeight,

			IsTerminated:   newAmount == "0",
			NeutronAddress: nil,
		})
	}

	// Create new position if amount is not zero
	if newAmount != "0" {
		updates = append(updates, PositionUpdate{
			EthereumAddress:     ethereumAddress,
			ContractAddress:     event.Log.Address.Hex(),
			Amount:              newAmount,
			PositionStartHeight: event.Log.BlockNumber,
			PositionEndHeight:   nil,

			IsTerminated:   false,
			NeutronAddress: nil,
		})
	}

	return updates, nil
}
