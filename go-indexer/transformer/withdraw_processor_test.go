package transformer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

func TestProcessWithdrawRequest(t *testing.T) {

	t.Run("partial withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var currentPosition = database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
		}
		var event = PositionEvent{
			EventName: "WithdrawRequested",
			EventData: map[string]interface{}{
				"owner":    common.HexToAddress(UserAddress1),
				"shares":   big.NewInt(50),
				"receiver": NeutronAddress1,
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, &currentPosition, nil, maxPositionIndexId)

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                      currentPosition.Id,
				PositionEndHeight:       toInt64Ptr(1999),
				IsTerminated:            toBoolPtr(false),
				WithdrawReceiverAddress: toStringPtr(NeutronAddress1),
			},
		}
		var expectedInserts = []database.PublicPositionsInsert{
			{
				OwnerAddress:        UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
				PositionStartHeight: 2000,
				PositionEndHeight:   nil,
				IsTerminated:        nil,
				PositionIndexId:     101,
			},
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})

	t.Run("full withdraw", func(t *testing.T) {
		processor := &PositionProcessor{}
		var senderPosition = database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
		}
		var event = PositionEvent{
			EventName: "WithdrawRequested",
			EventData: map[string]interface{}{
				"owner":    common.HexToAddress(UserAddress1),
				"shares":   big.NewInt(100),
				"receiver": NeutronAddress1,
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, &senderPosition, nil, maxPositionIndexId)

		var expectedInserts = []database.PublicPositionsInsert(nil)
		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                      senderPosition.Id,
				PositionEndHeight:       toInt64Ptr(1999),
				IsTerminated:            toBoolPtr(true),
				WithdrawReceiverAddress: toStringPtr(NeutronAddress1),
			},
		}

		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
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
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, nil, nil, maxPositionIndexId)
		assert.NoError(t, err)
		assert.Equal(t, []database.PublicPositionsInsert(nil), gotInserts)
		assert.Equal(t, []database.PublicPositionsUpdate(nil), gotUpdates)
	})
}
