package extractor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// EventData represents the raw event data map
type EventData map[string]interface{}

// TransferData represents the extracted data from a Transfer event
type TransferData struct {
	From   string
	To     string
	Amount string
}

// WithdrawData represents the extracted data from a WithdrawRequested event
type WithdrawData struct {
	Owner      string
	Receiver   string
	Amount     string
	WithdrawID string
}

// EventExtractor handles extracting data from different event types
type EventExtractor struct{}

// NewEventExtractor creates a new EventExtractor instance
func NewEventExtractor() *EventExtractor {
	return &EventExtractor{}
}

// ParseEventData parses the base64 encoded JSON data from an event
func (e *EventExtractor) ParseEventData(rawData interface{}) (EventData, error) {
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
	var data EventData
	if err := json.Unmarshal(decoded, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return data, nil
}

// ExtractTransfer extracts data from a Transfer event
func (e *EventExtractor) ExtractTransfer(data EventData) (*TransferData, error) {
	to, ok := data["to"].(string)
	if !ok {
		return nil, errors.New("to not found in transfer event")
	}

	from, ok := data["from"].(string)
	if !ok {
		return nil, errors.New("from not found in transfer event")
	}

	value, ok := data["value"].(float64)
	if !ok {
		return nil, errors.New("value not found in transfer event")
	}

	return &TransferData{
		From:   from,
		To:     to,
		Amount: fmt.Sprintf("%.0f", value),
	}, nil
}

// ExtractWithdraw extracts data from a WithdrawRequested event
func (e *EventExtractor) ExtractWithdraw(data EventData) (*WithdrawData, error) {
	owner, ok := data["owner"].(string)
	if !ok {
		return nil, errors.New("owner not found in withdraw event")
	}

	receiver, ok := data["receiver"].(string)
	if !ok {
		return nil, errors.New("receiver not found in withdraw event")
	}

	shares, ok := data["shares"].(float64)
	if !ok {
		return nil, errors.New("shares not found in withdraw event")
	}

	id, ok := data["id"].(float64)
	if !ok {
		return nil, errors.New("id not found in withdraw event")
	}

	return &WithdrawData{
		Owner:      owner,
		Receiver:   receiver,
		Amount:     fmt.Sprintf("%.0f", shares),
		WithdrawID: fmt.Sprintf("%.0f", id),
	}, nil
}
