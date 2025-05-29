package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
)

// EventProcessor handles blockchain event processing and storage
type EventProcessor struct {
	db     *supa.Client
	client *ethclient.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(db *supa.Client, client *ethclient.Client) *EventProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventProcessor{
		db:     db,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (e *EventProcessor) Stop() {
	e.cancel()
}

func (e *EventProcessor) ProcessEvent(vLog types.Log, event abi.Event, contractName string) (map[string]interface{}, error) {
	// Parse the event data
	eventData := make(map[string]interface{})

	// TODO: potentianl issue if non-indexed parameters come before indexed parameters. It will shift the offset.

	// Handle indexed parameters (topics)
	for i, input := range event.Inputs {
		if input.Indexed {
			// Skip the first topic as it's the event signature
			if i+1 < len(vLog.Topics) {
				// For indexed parameters, we need to handle them differently based on their type
				var value interface{}
				var err error

				switch input.Type.T {
				case abi.AddressTy:
					value = common.BytesToAddress(vLog.Topics[i+1].Bytes())
				case abi.IntTy, abi.UintTy:
					value = new(big.Int).SetBytes(vLog.Topics[i+1].Bytes())
				case abi.BoolTy:
					value = vLog.Topics[i+1].Bytes()[0] != 0
				case abi.BytesTy, abi.FixedBytesTy:
					value = vLog.Topics[i+1].Bytes()
				case abi.StringTy:
					value = string(vLog.Topics[i+1].Bytes())
				default:
					return nil, fmt.Errorf("unsupported indexed parameter type: %v", input.Type)
				}

				if err != nil {
					return nil, fmt.Errorf("failed to parse indexed parameter %s: %w", input.Name, err)
				}
				eventData[input.Name] = value
			}
		}
	}

	// Handle non-indexed parameters (data)
	if err := event.Inputs.UnpackIntoMap(eventData, vLog.Data); err != nil {
		return nil, fmt.Errorf("failed to unpack event data: %w", err)
	}

	// Convert event data to JSON for storage
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Create the event record matching the database schema
	eventRecord := map[string]interface{}{
		"contract_address": vLog.Address.Hex(),
		"event_name":       event.Name,
		"block_number":     vLog.BlockNumber,
		"transaction_hash": vLog.TxHash.Hex(),
		"log_index":        vLog.Index,
		"raw_data":         eventJSON,
	}

	// Insert into Supabase
	_, _, err = e.db.From("events").Insert(eventRecord, false, "", "", "").Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to insert event into database: %w", err)
	}
	log.Printf("Inserted event for event: %s, tx: %s, block: %d, data: %s", event.Name, vLog.TxHash.Hex(), vLog.BlockNumber, string(eventJSON))

	return eventData, nil
}
