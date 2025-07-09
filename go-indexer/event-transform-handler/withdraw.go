package eventTransformHandler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/timewave/vault-indexer/go-indexer/database"
	transformers "github.com/timewave/vault-indexer/go-indexer/transformers"
)

// WithdrawHandler handles WithdrawRequested events
type WithdrawHandler struct {
	positionTransformer        *transformers.PositionTransformer
	withdrawRequestTransformer *transformers.WithdrawRequestTransformer
}

// NewWithdrawHandler creates a new withdraw handler
func NewWithdrawHandler(positionTransformer *transformers.PositionTransformer, withdrawRequestTransformer *transformers.WithdrawRequestTransformer) *WithdrawHandler {
	return &WithdrawHandler{
		positionTransformer:        positionTransformer,
		withdrawRequestTransformer: withdrawRequestTransformer,
	}
}

// WithdrawData represents the extracted data from a WithdrawRequested event
type WithdrawData struct {
	Owner      string
	Receiver   string
	Amount     string
	WithdrawID string
}

// extractWithdraw extracts data from a WithdrawRequested event
func (h *WithdrawHandler) extract(data database.EventData) (*WithdrawData, error) {
	owner, ok := data["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner not found in withdraw event")
	}

	receiver, ok := data["receiver"].(string)
	if !ok {
		return nil, fmt.Errorf("receiver not found in withdraw event")
	}

	shares, ok := data["shares"].(float64)
	if !ok {
		return nil, fmt.Errorf("shares not found in withdraw event")
	}

	id, ok := data["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("id not found in withdraw event")
	}

	return &WithdrawData{
		Owner:      owner,
		Receiver:   receiver,
		Amount:     fmt.Sprintf("%.0f", shares),
		WithdrawID: fmt.Sprintf("%.0f", id),
	}, nil
}

// Handle processes a WithdrawRequested event
func (h *WithdrawHandler) Handle(event database.PublicEventsSelect, eventData database.EventData) (database.DatabaseOperations, error) {
	var operations database.DatabaseOperations

	withdrawData, err := h.extract(eventData)
	if err != nil {
		return operations, err
	}

	withdrawIdNum, err := strconv.ParseInt(withdrawData.WithdrawID, 10, 64)
	if err != nil {
		return operations, err
	}

	// Process position transformation
	inserts, updates, err := h.positionTransformer.Withdraw(transformers.ProcessPosition{
		ReceiverAddress: strings.ToLower(withdrawData.Receiver),
		SenderAddress:   strings.ToLower(withdrawData.Owner),
		ContractAddress: strings.ToLower(event.ContractAddress),
		AmountShares:    withdrawData.Amount,
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

	// Process withdraw request transformation
	withdrawRequest, err := h.withdrawRequestTransformer.Transform(transformers.ProcessWithdrawRequest{
		OwnerAddress:    withdrawData.Owner,
		ReceiverAddress: withdrawData.Receiver,
		Amount:          withdrawData.Amount,
		WithdrawId:      withdrawIdNum,
		BlockNumber:     uint64(event.BlockNumber),
		ContractAddress: event.ContractAddress,
	})

	if err != nil {
		return operations, err
	}

	operations.Inserts = append(operations.Inserts, database.DBOperation{
		Table: "withdraw_requests",
		Data:  withdrawRequest,
	})

	return operations, nil
}
