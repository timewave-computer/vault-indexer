package indexer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

const VaultAddress = "0x0000000000000000000000000000000000000456"
const UserAddress1 = "0x0000000000000000000000000000000000000123"

func toUint64Ptr(n int) *uint64 {
	v := uint64(n)
	return &v
}

func stringPtr(s string) *string {
	return &s
}

func TestProcessDeposit(t *testing.T) {

	t.Run("deposit, no existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Deposit",
			EventData: map[string]interface{}{
				"sender": common.HexToAddress(UserAddress1),
				"assets": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 1000,
			},
		}
		var expectedUpdates = []PositionUpdate{
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				Amount:              "50",
				PositionStartHeight: 1000,
				PositionEndHeight:   nil,
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
		var currentPosition = Position{
			ID:                1,
			Amount:            "100",
			PositionEndHeight: nil,

			IsTerminated:        false,
			NeutronAddress:      nil,
			PositionStartHeight: 1000,
			EthereumAddress:     UserAddress1,
			ContractAddress:     VaultAddress,
		}
		var event = PositionEvent{
			EventName: "Deposit",
			EventData: map[string]interface{}{
				"sender": common.HexToAddress(UserAddress1),
				"assets": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		var expectedUpdates = []PositionUpdate{
			{
				Id:                  &currentPosition.ID,
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				Amount:              "100",
				PositionStartHeight: 1000,
				PositionEndHeight:   toUint64Ptr(1999),
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				Amount:              "150",
				PositionStartHeight: 2000,
				PositionEndHeight:   nil,
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
		}
		gotUpdates, err := processor.processPositionEvent(event, &currentPosition)
		assert.NoError(t, err)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}

func TestProcessWithdraw(t *testing.T) {

	// TODO: handle no existing position

	t.Run("partial withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = Position{
			ID:                  1,
			Amount:              "100",
			PositionEndHeight:   nil,
			IsTerminated:        false,
			NeutronAddress:      nil,
			PositionStartHeight: 1000,
			EthereumAddress:     UserAddress1,
			ContractAddress:     VaultAddress,
		}
		var event = PositionEvent{
			EventName: "Withdraw",
			EventData: map[string]interface{}{
				"sender": common.HexToAddress(UserAddress1),
				"assets": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		var expectedUpdates = []PositionUpdate{
			{
				Id:                  &currentPosition.ID,
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				Amount:              "50",
				PositionStartHeight: 1000,
				PositionEndHeight:   toUint64Ptr(1999),
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
			{
				Id:                  &currentPosition.ID,
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				Amount:              "50",
				PositionStartHeight: 1000,
				PositionEndHeight:   toUint64Ptr(1999),
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
		}
		gotUpdates, err := processor.processPositionEvent(event, &currentPosition)
		assert.NoError(t, err)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}
