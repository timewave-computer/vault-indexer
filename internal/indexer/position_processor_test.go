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
const NeutronAddress1 = "neutron14wey3cpz2cxswu9u6gaalz2xxh03xdeyqal877"

func toUint64Ptr(n int) *uint64 {
	v := uint64(n)
	return &v
}

func toStringPtr(s string) *string {
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
				AmountShares:        "50",
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
			AmountShares:      "100",
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
				AmountShares:        "100",
				PositionStartHeight: 1000,
				PositionEndHeight:   toUint64Ptr(1999),
				IsTerminated:        false,
				NeutronAddress:      nil,
			},
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "150",
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

	t.Run("partial withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = Position{
			ID:                  1,
			AmountShares:        "100",
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
				"sender":   common.HexToAddress(UserAddress1),
				"assets":   big.NewInt(50),
				"receiver": NeutronAddress1,
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
				AmountShares:        "100",
				PositionStartHeight: 1000,
				PositionEndHeight:   toUint64Ptr(1999),
				IsTerminated:        false,
				NeutronAddress:      toStringPtr(NeutronAddress1),
			},
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
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

	t.Run("full withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = Position{
			ID:                  1,
			AmountShares:        "100",
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
				"sender":   common.HexToAddress(UserAddress1),
				"assets":   big.NewInt(100),
				"receiver": NeutronAddress1,
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
				AmountShares:        "100",
				PositionStartHeight: 1000,
				PositionEndHeight:   toUint64Ptr(1999),
				IsTerminated:        true,
				NeutronAddress:      toStringPtr(NeutronAddress1),
			},
		}
		gotUpdates, err := processor.processPositionEvent(event, &currentPosition)
		assert.NoError(t, err)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})

	t.Run("no existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Withdraw",
			EventData: map[string]interface{}{
				"sender":   common.HexToAddress(UserAddress1),
				"assets":   big.NewInt(100),
				"receiver": NeutronAddress1,
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		var expectedUpdates = []PositionUpdate{}
		gotUpdates, err := processor.processPositionEvent(event, nil)
		assert.NoError(t, err)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}
