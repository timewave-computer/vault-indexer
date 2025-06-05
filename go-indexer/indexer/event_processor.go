package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	supa "github.com/supabase-community/supabase-go"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// EventProcessor handles blockchain event processing and storage
type EventProcessor struct {
	db     *supa.Client
	client *ethclient.Client
	ctx    context.Context
	cancel context.CancelFunc
	logger *logger.Logger
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(db *supa.Client, client *ethclient.Client) *EventProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventProcessor{
		db:     db,
		client: client,
		ctx:    ctx,
		cancel: cancel,
		logger: logger.NewLogger("EventProcessor"),
	}
}

func (e *EventProcessor) Stop() {
	e.cancel()
}

func (e *EventProcessor) processEvent(vLog types.Log, event abi.Event, contractName string) error {
	eventData, err := parseEvent(vLog, event, contractName)
	if err != nil {
		return fmt.Errorf("failed to process event: %w", err)
	}

	blockNumber := eventData.BlockNumber
	logIndex := int64(eventData.LogIndex)

	data, _, err := e.db.From("events").
		Select("id", "", false).
		Eq("block_number", strconv.FormatInt(blockNumber, 10)).
		Eq("log_index", strconv.FormatInt(logIndex, 10)).Execute()

	if err != nil {
		return fmt.Errorf("failed to get event from database: %w", err)
	}

	var response []struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal database response: %w", err)
	}

	if len(response) > 0 {
		eventId := response[0].Id
		e.logger.Printf("Updating existing event: %v", eventId)

		now := "now()"
		_, _, err = e.db.From("events").Update(ToEventIngestionUpdate(database.PublicEventsUpdate{
			LastUpdatedAt: &now,
		}), "", "").Eq("id", eventId).Execute()

		if err != nil {
			return fmt.Errorf("failed to update event in database: %w", err)
		}
		return nil
	} else {
		// Insert into Supabase
		_, _, err = e.db.From("events").Insert(eventData, false, "", "", "").Execute()
		if err != nil {
			return fmt.Errorf("failed to insert event into database: %w", err)
		}
		return nil
	}
}

func parseEvent(vLog types.Log, event abi.Event, contractName string) (*EventIngestionInsert, error) {
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

	eventRecord := ToEventIngestionInsert(database.PublicEventsInsert{
		ContractAddress: vLog.Address.Hex(),
		EventName:       event.Name,
		BlockNumber:     int64(vLog.BlockNumber),
		TransactionHash: vLog.TxHash.Hex(),
		LogIndex:        int32(vLog.Index),
		RawData:         eventJSON,
	})

	return &eventRecord, nil
}

type EventIngestionUpdate struct {
	LastUpdatedAt *string `json:"last_updated_at"`
}

func ToEventIngestionUpdate(u database.PublicEventsUpdate) EventIngestionUpdate {

	// omits empty values so they are not attempted to be updated

	return EventIngestionUpdate{
		LastUpdatedAt: u.LastUpdatedAt,
	}
}

type EventIngestionInsert struct {
	ContractAddress string      `json:"contract_address"`
	EventName       string      `json:"event_name"`
	BlockNumber     int64       `json:"block_number"`
	TransactionHash string      `json:"transaction_hash"`
	LogIndex        int32       `json:"log_index"`
	RawData         interface{} `json:"raw_data"`
}

func ToEventIngestionInsert(u database.PublicEventsInsert) EventIngestionInsert {

	// omits empty values so they are not attempted to be updated

	return EventIngestionInsert{
		ContractAddress: u.ContractAddress,
		EventName:       u.EventName,
		BlockNumber:     u.BlockNumber,
		TransactionHash: u.TransactionHash,
		LogIndex:        u.LogIndex,
		RawData:         u.RawData,
	}
}
