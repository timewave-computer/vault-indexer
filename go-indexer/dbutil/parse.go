package dbutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/timewave/vault-indexer/go-indexer/database"
)

func ParseQuotedBased64Json(event database.PublicEventsSelect) (map[string]interface{}, error) {

	rawBytes, ok := event.RawData.([]uint8)
	if !ok {
		return nil, fmt.Errorf("raw data is not of type []uint8")
	}
	base64string := string(rawBytes)
	unquoted, err := strconv.Unquote(base64string)
	if err != nil {
		return nil, fmt.Errorf("failed to unquote raw data: %w", err)
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(unquoted)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(decodedBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw data: %w", err)
	}
	return data, nil

}
