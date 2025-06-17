package eventTransformHandler

import (
	"fmt"

	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/transformer"
)

// TransferHandler handles Transfer events
type TransferHandler struct {
	positionTransformer *transformer.PositionTransformer
}

// NewTransferHandler creates a new transfer handler
func NewTransferHandler(positionTransformer *transformer.PositionTransformer) *TransferHandler {
	return &TransferHandler{
		positionTransformer: positionTransformer,
	}
}

// TransferData represents the extracted data from a Transfer event
type TransferData struct {
	From   string
	To     string
	Amount string
}

// extractTransfer extracts data from a Transfer event
func (h *TransferHandler) extract(data database.EventData) (*TransferData, error) {
	to, ok := data["to"].(string)
	if !ok {
		return nil, fmt.Errorf("to not found in transfer event")
	}

	from, ok := data["from"].(string)
	if !ok {
		return nil, fmt.Errorf("from not found in transfer event")
	}

	value, ok := data["value"].(float64)
	if !ok {
		return nil, fmt.Errorf("value not found in transfer event")
	}

	return &TransferData{
		From:   from,
		To:     to,
		Amount: fmt.Sprintf("%.0f", value),
	}, nil
}

// Handle processes a Transfer event
func (h *TransferHandler) Handle(event database.PublicEventsSelect, eventData database.EventData) (database.DatabaseOperations, error) {
	var operations database.DatabaseOperations

	transferData, err := h.extract(eventData)
	if err != nil {
		return operations, err
	}

	inserts, updates, err := h.positionTransformer.Transfer(transformer.ProcessPosition{
		ReceiverAddress: transferData.To,
		SenderAddress:   transferData.From,
		ContractAddress: event.ContractAddress,
		AmountShares:    transferData.Amount,
		BlockNumber:     uint64(event.BlockNumber),
	})

	if err != nil {
		return operations, err
	}

	if len(inserts) > 0 {
		operations.Inserts = append(operations.Inserts, database.DBOperation{
			Table: "positions",
			Data:  inserts,
		})
	}
	if len(updates) > 0 {
		operations.Updates = append(operations.Updates, database.DBOperation{
			Table: "positions",
			Data:  updates,
		})
	}

	return operations, nil
}
