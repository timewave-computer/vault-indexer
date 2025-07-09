package eventTransformHandler

import (
	"fmt"
	"strings"

	"github.com/timewave/vault-indexer/go-indexer/database"
	transformers "github.com/timewave/vault-indexer/go-indexer/transformers"
)

type RateUpdateHandler struct {
	rateUpdateTransformer *transformers.RateUpdateTransformer
}

func NewRateUpdateHandler(rateUpdateTransformer *transformers.RateUpdateTransformer) *RateUpdateHandler {
	return &RateUpdateHandler{
		rateUpdateTransformer: rateUpdateTransformer,
	}
}

// WithdrawData represents the extracted data from a WithdrawRequested event
type RateUpdateData struct {
	Rate string
}

// extractWithdraw extracts data from a WithdrawRequested event
func (h *RateUpdateHandler) extract(data database.EventData) (*RateUpdateData, error) {
	newRate, ok := data["newRate"].(float64)
	if !ok {
		return nil, fmt.Errorf("newRate not found in rate update event")
	}

	return &RateUpdateData{
		Rate: fmt.Sprintf("%.0f", newRate),
	}, nil
}

// Handle processes a WithdrawRequested event
func (h *RateUpdateHandler) Handle(event database.PublicEventsSelect, eventData database.EventData) (database.DatabaseOperations, error) {
	var operations database.DatabaseOperations

	rateUpdateData, err := h.extract(eventData)

	if err != nil {
		return operations, err
	}

	// Process position transformation
	inserts, err := h.rateUpdateTransformer.Transform(transformers.ProcessRateUpdate{
		ContractAddress: strings.ToLower(event.ContractAddress),
		Rate:            rateUpdateData.Rate,
		BlockNumber:     event.BlockNumber,
	})

	if err != nil {
		return operations, err
	}

	operations.Inserts = append(operations.Inserts, database.DBOperation{
		Table: "rate_updates",
		Data:  inserts,
	})

	return operations, nil
}
