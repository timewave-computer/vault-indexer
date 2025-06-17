package eventTransformHandler

import (
	"fmt"

	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/transformer"
)

// WithdrawHandler handles WithdrawRequested events
type DepositHandler struct {
	positionTransformer *transformer.PositionTransformer
}

// NewWithdrawHandler creates a new withdraw handler
func NewDepositHandler(positionTransformer *transformer.PositionTransformer) *DepositHandler {
	return &DepositHandler{
		positionTransformer: positionTransformer,
	}
}

// WithdrawData represents the extracted data from a WithdrawRequested event
type DepositData struct {
	Sender       string
	Owner        string
	AmountShares string
}

// extractWithdraw extracts data from a WithdrawRequested event
func (h *DepositHandler) extract(data database.EventData) (*DepositData, error) {
	owner, ok := data["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner not found in withdraw event")
	}

	sender, ok := data["sender"].(string)
	if !ok {
		return nil, fmt.Errorf("sender not found in withdraw event")
	}

	shares, ok := data["shares"].(float64)
	if !ok {
		return nil, fmt.Errorf("shares not found in withdraw event")
	}

	return &DepositData{
		Owner:        owner,
		Sender:       sender,
		AmountShares: fmt.Sprintf("%.0f", shares),
	}, nil
}

// Handle processes a WithdrawRequested event
func (h *DepositHandler) Handle(event database.PublicEventsSelect, eventData database.EventData) (database.DatabaseOperations, error) {
	var operations database.DatabaseOperations

	depositData, err := h.extract(eventData)
	if err != nil {
		return operations, err
	}

	// Process position transformation
	inserts, updates, err := h.positionTransformer.Deposit(transformer.ProcessPosition{
		SenderAddress:   depositData.Sender,
		ContractAddress: event.ContractAddress,
		AmountShares:    depositData.AmountShares,
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
