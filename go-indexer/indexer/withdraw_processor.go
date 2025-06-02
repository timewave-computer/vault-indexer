package indexer

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
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
	wg     sync.WaitGroup
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

func (p *WithdrawProcessor) Start(events <-chan WithdrawRequestEvent) error {
	log.Println("Starting withdraw request processor...")
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
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

	var withdrawId int64
	var ethereumAddress string
	var contractAddress = event.Log.Address.Hex()
	var amount string

	// Extract required fields from event data
	id, ok := event.EventData["id"]
	if !ok {
		return fmt.Errorf("invalid withdrawId in event data")
	}

	// Handle *big.Int type for id
	if bigInt, ok := id.(*big.Int); ok {
		withdrawId = bigInt.Int64()
	} else {
		return fmt.Errorf("invalid withdrawId type in event data")
	}

	if addr, ok := event.EventData["owner"].(common.Address); ok {
		ethereumAddress = addr.Hex()
	} else {
		return fmt.Errorf("invalid ethereumAddress in event data")
	}

	if value, ok := event.EventData["shares"].(*big.Int); ok {
		amount = value.String()
	} else {
		return fmt.Errorf("invalid amount in event data")
	}

	neutronAddress, ok := event.EventData["receiver"].(string)
	if !ok {
		return fmt.Errorf("invalid neutronAddress in event data")
	}

	// Insert withdraw request into database
	_, _, err := p.db.From("withdraw_requests").Insert(database.PublicWithdrawRequestsInsert{
		WithdrawId:      withdrawId,
		ContractAddress: contractAddress,
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
	p.wg.Wait()
}
