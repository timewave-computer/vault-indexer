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
				if err := p.processPositionEvent(event); err != nil {
					log.Printf("Error processing position event: %v", err)
					// Block processing on error as requested
					// In a production environment, you might want to add retry logic or alerting
					return
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

func (p *PositionProcessor) processPositionEvent(event PositionEvent) error {
	var ethereumAddress string
	var amount string
	var entryMethod string
	var exitMethod string

	// Handle different event types
	switch event.EventName {
	case "Deposit":
		if owner, ok := event.EventData["sender"].(common.Address); ok {
			ethereumAddress = owner.Hex()
		}
		if assets, ok := event.EventData["assets"].(*big.Int); ok {
			amount = assets.String()
		}
		entryMethod = "deposit"
		exitMethod = "deposit"

	case "Transfer":
		if to, ok := event.EventData["from"].(common.Address); ok {
			ethereumAddress = to.Hex()
		}
		if value, ok := event.EventData["value"].(*big.Int); ok {
			amount = value.String()
		}
		entryMethod = "transfer"
		exitMethod = "transfer"

	case "Withdraw":
		if owner, ok := event.EventData["sender"].(common.Address); ok {
			ethereumAddress = owner.Hex()
		}
		if assets, ok := event.EventData["assets"].(*big.Int); ok {
			// For withdrawals, we'll store the negative value as a string
			negAssets := new(big.Int).Neg(assets)
			amount = negAssets.String()
		}
		exitMethod = "withdraw"
	default:
		return nil
	}

	if ethereumAddress == "" {
		return fmt.Errorf("could not determine account address from event data")
	}

	// Get current position if it exists
	var currentPosition struct {
		PositionIndexNumber int64       `json:"position_index_number"`
		Amount              json.Number `json:"amount"`
		PositionEndHeight   *int64      `json:"position_end_height"`
	}
	data, _, err := p.db.From("positions").
		Select("position_index_number,amount,position_end_height", "", false).
		Eq("ethereum_address", ethereumAddress).
		Eq("contract_address", event.Log.Address.Hex()).
		Is("position_end_height", "null").
		Single().
		Execute()

	if err == nil {
		if err := json.Unmarshal(data, &currentPosition); err != nil {
			return fmt.Errorf("failed to unmarshal current position: %w", err)
		}
	}

	// Calculate new amount
	newAmount := amount
	if err == nil {
		// Add the current amount to the new amount using big.Int
		currentBigInt := new(big.Int)
		currentBigInt.SetString(currentPosition.Amount.String(), 10)
		newBigInt := new(big.Int)
		newBigInt.SetString(amount, 10)
		newBigInt.Add(currentBigInt, newBigInt)
		newAmount = newBigInt.String()
	}

	// Close current position if it exists
	if err == nil {
		endHeight := int64(event.Log.BlockNumber - 1)
		_, _, err = p.db.From("positions").
			Update(map[string]interface{}{
				"position_end_height": endHeight,
				"exit_method":         exitMethod,
				"is_terminated":       newAmount == "0",
			}, "", "").
			Eq("position_index_number", fmt.Sprintf("%d", currentPosition.PositionIndexNumber)).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to close current position: %w", err)
		}
	}

	// Create new position if amount is not zero
	if newAmount != "0" {
		positionRecord := map[string]interface{}{
			"ethereum_address":      ethereumAddress,
			"contract_address":      event.Log.Address.Hex(),
			"amount":                newAmount,
			"position_start_height": event.Log.BlockNumber,
			"position_end_height":   nil,
			"entry_method":          entryMethod,
			"exit_method":           nil,
			"is_terminated":         false,
			"neutron_address":       nil,
		}

		_, _, err = p.db.From("positions").Insert(positionRecord, false, "", "", "").Execute()
		if err != nil {
			return fmt.Errorf("failed to create new position: %w", err)
		}

		log.Printf("Created new position for account %s: amount = %s", positionRecord["ethereum_address"], positionRecord["amount"])
	}

	return nil
}
