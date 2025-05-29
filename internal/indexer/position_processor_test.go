package indexer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func TestProcessPositionEvent(t *testing.T) {

	t.Run("deposit, no existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Deposit",
			EventData: map[string]interface{}{
				"sender": common.HexToAddress("0000000000000000000000000000000000000123"),
				"assets": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress("0000000000000000000000000000000000000456"),
				BlockNumber: 1000,
			},
		}
		var expectedUpdates = []PositionUpdate{
			{
				EthereumAddress:     "0x0000000000000000000000000000000000000123",
				ContractAddress:     "0x0000000000000000000000000000000000000456",
				Amount:              "50",
				PositionStartHeight: 1000,
				PositionEndHeight:   nil,
				EntryMethod:         "deposit",
				ExitMethod:          nil,
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
		}
		gotUpdates, err := processor.processPositionEvent(event, nil)
		assert.NoError(t, err)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
	t.Run("deposit, existing position ", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Deposit",
			EventData: map[string]interface{}{
				"sender": common.HexToAddress("0000000000000000000000000000000000000123"),
				"assets": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress("0000000000000000000000000000000000000456"),
				BlockNumber: 1000,
			},
		}
		var expectedUpdates = []PositionUpdate{
			{
				EthereumAddress:     "0x0000000000000000000000000000000000000123",
				ContractAddress:     "0x0000000000000000000000000000000000000456",
				Amount:              "50",
				PositionStartHeight: 1000,
				PositionEndHeight:   nil,
				EntryMethod:         "deposit",
				ExitMethod:          nil,
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
		}
		gotUpdates, err := processor.processPositionEvent(event, nil)
		assert.NoError(t, err)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}
