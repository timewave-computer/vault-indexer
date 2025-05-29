package indexer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/timewave/vault-indexer/internal/database"
)

const VaultAddress = "0x0000000000000000000000000000000000000456"
const UserAddress1 = "0x0000000000000000000000000000000000000123"
const NeutronAddress1 = "neutron14wey3cpz2cxswu9u6gaalz2xxh03xdeyqal877"

func toStringPtr(s string) *string {
	return &s
}

func toBoolPtr(b bool) *bool {
	return &b
}

func toInt64Ptr(n int) *int64 {
	v := int64(n)
	return &v
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
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, nil)

		var expectedInserts = []database.PublicPositionsInsert{
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
				PositionStartHeight: 1000,
				PositionEndHeight:   nil,
				IsTerminated:        toBoolPtr(false),
				NeutronAddress:      nil,
			},
		}

		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, []database.PublicPositionsUpdate(nil), gotUpdates)
	})

	t.Run("deposit, existing position ", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = database.PublicPositionsSelect{
			Id:                  1,
			AmountShares:        "100",
			PositionEndHeight:   nil,
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
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, &currentPosition)

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                  &currentPosition.Id,
				EthereumAddress:     toStringPtr(UserAddress1),
				ContractAddress:     toStringPtr(VaultAddress),
				AmountShares:        toStringPtr("100"),
				PositionStartHeight: toInt64Ptr(1000),
				PositionEndHeight:   toInt64Ptr(1999),
				IsTerminated:        toBoolPtr(false),
				NeutronAddress:      nil,
			},
		}
		var expectedInserts = []database.PublicPositionsInsert{
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "150",
				PositionStartHeight: 2000,
				PositionEndHeight:   nil,
				IsTerminated:        toBoolPtr(false),
				NeutronAddress:      nil,
			},
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}

func TestProcessWithdraw(t *testing.T) {

	t.Run("partial withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = database.PublicPositionsSelect{
			Id:                  1,
			AmountShares:        "100",
			PositionEndHeight:   nil,
			IsTerminated:        false,
			NeutronAddress:      nil,
			PositionStartHeight: 1000,
			EthereumAddress:     UserAddress1,
			ContractAddress:     VaultAddress,
		}
		var event = PositionEvent{
			EventName: "WithdrawRequested",
			EventData: map[string]interface{}{
				"owner":    common.HexToAddress(UserAddress1),
				"assets":   big.NewInt(50),
				"receiver": NeutronAddress1,
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, &currentPosition)

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                  &currentPosition.Id,
				EthereumAddress:     toStringPtr(UserAddress1),
				ContractAddress:     toStringPtr(VaultAddress),
				AmountShares:        toStringPtr("100"),
				PositionStartHeight: toInt64Ptr(1000),
				PositionEndHeight:   toInt64Ptr(1999),
				IsTerminated:        toBoolPtr(false),
				NeutronAddress:      toStringPtr(NeutronAddress1),
			},
		}
		var expectedInserts = []database.PublicPositionsInsert{
			{
				EthereumAddress:     UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
				PositionStartHeight: 2000,
				PositionEndHeight:   nil,
				IsTerminated:        toBoolPtr(false),
			},
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})

	t.Run("full withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = database.PublicPositionsSelect{
			Id:                  1,
			AmountShares:        "100",
			PositionEndHeight:   nil,
			IsTerminated:        false,
			NeutronAddress:      nil,
			PositionStartHeight: 1000,
			EthereumAddress:     UserAddress1,
			ContractAddress:     VaultAddress,
		}
		var event = PositionEvent{
			EventName: "WithdrawRequested",
			EventData: map[string]interface{}{
				"owner":    common.HexToAddress(UserAddress1),
				"assets":   big.NewInt(100),
				"receiver": NeutronAddress1,
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, &currentPosition)

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                  &currentPosition.Id,
				EthereumAddress:     toStringPtr(UserAddress1),
				ContractAddress:     toStringPtr(VaultAddress),
				AmountShares:        toStringPtr("100"),
				PositionStartHeight: toInt64Ptr(1000),
				PositionEndHeight:   toInt64Ptr(1999),
				IsTerminated:        toBoolPtr(true),
				NeutronAddress:      toStringPtr(NeutronAddress1),
			},
		}

		assert.NoError(t, err)
		assert.Equal(t, []database.PublicPositionsInsert(nil), gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})

	t.Run("no existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "WithdrawRequested",
			EventData: map[string]interface{}{
				"owner":    common.HexToAddress(UserAddress1),
				"assets":   big.NewInt(100),
				"receiver": NeutronAddress1,
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, nil)
		assert.NoError(t, err)
		assert.Equal(t, []database.PublicPositionsInsert(nil), gotInserts)
		assert.Equal(t, []database.PublicPositionsUpdate(nil), gotUpdates)
	})
}
