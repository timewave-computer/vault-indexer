package eventTransformHandler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/timewave/vault-indexer/go-indexer/database"
)

// Handler defines the interface for event handlers
type Handler interface {
	Handle(event database.PublicEventsSelect, eventData database.EventData) (database.DatabaseOperations, error)
}

// EventHandler is the main handler that routes events to specific handlers
type TransformHandler struct {
	transferHandler   *TransferHandler
	withdrawHandler   *WithdrawHandler
	rateUpdateHandler *RateUpdateHandler
}

// NewEventHandler creates a new event handler
func NewHandler(transferHandler *TransferHandler, withdrawHandler *WithdrawHandler, rateUpdateHandler *RateUpdateHandler) *TransformHandler {
	return &TransformHandler{
		transferHandler:   transferHandler,
		withdrawHandler:   withdrawHandler,
		rateUpdateHandler: rateUpdateHandler,
	}
}

// Handle routes events to the appropriate handler
func (h *TransformHandler) Handle(event database.PublicEventsSelect) (database.DatabaseOperations, error) {
	eventData, err := h.ParseEventData(event.RawData)
	if err != nil {
		return database.DatabaseOperations{}, err
	}

	switch event.EventName {
	case "Transfer":
		return h.transferHandler.Handle(event, eventData)
	case "WithdrawRequested":
		return h.withdrawHandler.Handle(event, eventData)
	case "RateUpdated":
		return h.rateUpdateHandler.Handle(event, eventData)
	default:
		return database.DatabaseOperations{}, nil
	}
}

func (e *TransformHandler) ParseEventData(rawData interface{}) (database.EventData, error) {
	var base64string string

	switch v := rawData.(type) {
	case string:
		base64string = v
	case []uint8:
		base64string = string(v)
	default:
		return nil, fmt.Errorf("unsupported raw data type: %T", rawData)
	}

	// Remove quotes if present
	base64string = strings.Trim(base64string, "\"")

	// Unquote if needed
	unquoted, err := strconv.Unquote(base64string)
	if err != nil {
		// If unquote fails, use the original string
		unquoted = base64string
	}

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(unquoted)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %v", err)
	}

	// Parse JSON
	var data database.EventData
	if err := json.Unmarshal(decoded, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return data, nil
}
