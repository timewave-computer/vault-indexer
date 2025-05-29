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
	"github.com/timewave/vault-indexer/internal/database"
)

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
				var currentPosition *database.PublicPositionsSelect
				var ethereumAddress string

				// Determine the ethereum address based on event type
				switch event.EventName {
				case "Deposit":
					if owner, ok := event.EventData["sender"].(common.Address); ok {
						ethereumAddress = owner.Hex()
					}
				case "Transfer":
					if to, ok := event.EventData["to"].(common.Address); ok {
						ethereumAddress = to.Hex()
					}
				case "WithdrawRequested":
					if owner, ok := event.EventData["owner"].(common.Address); ok {
						ethereumAddress = owner.Hex()
					}
				}

				if ethereumAddress == "" {
					log.Printf("Error: could not determine ethereum address from event data")
					continue
				}

				data, _, err := p.db.From("positions").
					Select("*", "", false).
					Eq("ethereum_address", ethereumAddress).
					Eq("contract_address", event.Log.Address.Hex()).
					Single().
					Execute()

				if err == nil {
					var pos database.PublicPositionsSelect
					if err := json.Unmarshal(data, &pos); err != nil {
						log.Printf("Error unmarshaling current position: %v", err)
						continue
					}
					currentPosition = &pos
				}

				// Process the event
				inserts, updates, err := p.processPositionEvent(event, currentPosition)
				if err != nil {
					log.Printf("Error processing position event: %v", err)
					continue
				}

				for _, insert := range inserts {
					_, _, err = p.db.From("positions").Insert(insert, false, "", "", "").Execute()
					if err != nil {
						log.Printf("Error inserting new position: %v", err)
						continue
					}
				}
				for _, update := range updates {
					_, _, err = p.db.From("positions").Update(update, "", "").Execute()
					if err != nil {
						log.Printf("Error updating position: %v", err)
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

func (p *PositionProcessor) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *PositionProcessor) processPositionEvent(event PositionEvent, currentPosition *database.PublicPositionsSelect) ([]database.PublicPositionsInsert, []database.PublicPositionsUpdate, error) {
	var ethereumAddress string
	var amount_shares string
	var neutronAddress *string
	// var isDeposit = false
	// var isWithdraw = false

	// Handle different event types
	switch event.EventName {
	case "Deposit":
		if owner, ok := event.EventData["sender"].(common.Address); ok {
			ethereumAddress = owner.Hex()
		}
		if assets, ok := event.EventData["assets"].(*big.Int); ok {
			amount_shares = assets.String()
		}

	case "WithdrawRequested":
		if currentPosition == nil {
			return nil, nil, nil
		}
		if owner, ok := event.EventData["owner"].(common.Address); ok {
			ethereumAddress = owner.Hex()
		}
		if receiver, ok := event.EventData["receiver"].(string); ok {
			neutronAddress = &receiver
		}
		if assets, ok := event.EventData["assets"].(*big.Int); ok {
			// For withdrawals, we'll store the negative value as a string
			negAssets := new(big.Int).Neg(assets)
			amount_shares = negAssets.String()
		}

	case "Transfer":
		if to, ok := event.EventData["from"].(common.Address); ok {
			ethereumAddress = to.Hex()
		}
		if value, ok := event.EventData["value"].(*big.Int); ok {
			amount_shares = value.String()
		}

		// Skip if from or to address is zero address
		from, fromOk := event.EventData["from"].(common.Address)
		to, toOk := event.EventData["to"].(common.Address)
		if (fromOk && from == common.HexToAddress("0x0000000000000000000000000000000000000000")) ||
			(toOk && to == common.HexToAddress("0x0000000000000000000000000000000000000000")) {
			return nil, nil, nil
		}

		// TODO: update 2 positions (from + to)

	default:
		return nil, nil, nil
	}

	if ethereumAddress == "" {
		return nil, nil, fmt.Errorf("could not determine account address from event data")
	}

	// Calculate new amount_shares
	newAmount := amount_shares
	if currentPosition != nil {
		// Add the current amount_shares to the new amount_shares using big.Int
		currentBigInt := new(big.Int)
		currentBigInt.SetString(currentPosition.AmountShares, 10)
		newBigInt := new(big.Int)
		newBigInt.SetString(amount_shares, 10)
		newBigInt.Add(currentBigInt, newBigInt)
		newAmount = newBigInt.String()
	}

	var updates []database.PublicPositionsUpdate
	var inserts []database.PublicPositionsInsert

	endHeight := int64(event.Log.BlockNumber - 1)
	isTerminated := newAmount == "0"
	contractAddr := event.Log.Address.Hex()
	blockNum := int64(event.Log.BlockNumber)

	// Close current position if it exists
	if currentPosition != nil {

		updates = append(updates, database.PublicPositionsUpdate{
			Id:                  &currentPosition.Id,
			EthereumAddress:     &ethereumAddress,
			ContractAddress:     &contractAddr,
			AmountShares:        &currentPosition.AmountShares,
			PositionStartHeight: &currentPosition.PositionStartHeight,
			PositionEndHeight:   &endHeight,
			IsTerminated:        &isTerminated,
			NeutronAddress:      neutronAddress,
		})
	}
	// Create new position if amount_shares is not zero
	if newAmount != "0" {
		newIsTerminated := false
		var newPosition = database.PublicPositionsInsert{
			EthereumAddress:     ethereumAddress,
			ContractAddress:     contractAddr,
			AmountShares:        newAmount,
			PositionStartHeight: blockNum,
			PositionEndHeight:   nil,
			IsTerminated:        &newIsTerminated,
			NeutronAddress:      nil,
		}
		inserts = append(inserts, newPosition)
	}

	return inserts, updates, nil
}
