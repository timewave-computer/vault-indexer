package indexer

import (
	"context"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/core/types"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/internal/database"
)

// WithdrawRequestEvent represents a withdraw request event from the blockchain
type WithdrawRequestEvent struct {
	EventName string
	EventData map[string]interface{}
	Log       types.Log
}

// WithdrawProcessor handles processing of withdraw request events
type WithdrawProcessor struct {
	db     *supa.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWithdrawProcessor creates a new withdraw processor
func NewWithdrawProcessor(db *supa.Client) *WithdrawProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &WithdrawProcessor{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins processing withdraw request events
func (p *WithdrawProcessor) Start(events <-chan WithdrawRequestEvent) error {
	go func() {
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				if err := p.processWithdrawRequest(event); err != nil {
					log.Printf("Error processing withdraw request: %v", err)
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
	return nil
}

// processWithdrawRequest processes a single withdraw request event
func (p *WithdrawProcessor) processWithdrawRequest(event WithdrawRequestEvent) error {
	// Extract required fields from event data
	withdrawID, ok := event.EventData["id"].(float64)
	if !ok {
		return fmt.Errorf("invalid withdrawId in event data")
	}

	ethereumAddress, ok := event.EventData["owner"].(string)
	if !ok {
		return fmt.Errorf("invalid ethereumAddress in event data")
	}

	amount, ok := event.EventData["shares"].(string)
	if !ok {
		return fmt.Errorf("invalid amount in event data")
	}

	neutronAddress, ok := event.EventData["reciever"].(string)
	if !ok {
		return fmt.Errorf("invalid neutronAddress in event data")
	}

	// Insert withdraw request into database
	_, _, err := p.db.From("withdraw_requests").Insert(database.PublicWithdrawRequestsInsert{
		WithdrawId:      int64(withdrawID),
		ContractAddress: event.Log.Address.Hex(),
		EthereumAddress: ethereumAddress,
		Amount:          amount,
		NeutronAddress:  neutronAddress,
	}, false, "", "", "").Execute()

	if err != nil {
		return fmt.Errorf("failed to insert withdraw request: %w", err)
	}

	return nil
}

// Stop stops the withdraw processor
func (p *WithdrawProcessor) Stop() {
	p.cancel()
}
