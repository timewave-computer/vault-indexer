package transformer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/timewave/vault-indexer/go-indexer/database"
)

const ZeroAddress = "0x0000000000000000000000000000000000000000"
const VaultAddress = "0x0000000000000000000000000000000000000456"
const UserAddress1 = "0x0000000000000000000000000000000000000123"
const UserAddress2 = "0x0000000000000000000000000000000000000789"
const NeutronAddress1 = "neutron14wey3cpz2cxswu9u6gaalz2xxh03xdeyqal877"
const PositionId1 = "aaa4d212-655b-4a7e-9af7-a93cee327eb4"
const PositionId2 = "bbb4d212-655b-4a7e-9af7-a93cee327eb4"
const maxPositionIndexId = 100

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
			EventName: "Transfer",
			EventData: map[string]interface{}{
				"from":  common.HexToAddress(ZeroAddress),
				"to":    common.HexToAddress(UserAddress1),
				"value": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 1000,
			},
		}
		var senderPosition *database.PublicPositionsSelect = nil
		var receiverPosition *database.PublicPositionsSelect = nil
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PublicPositionsInsert{
			{
				OwnerAddress:            UserAddress1,
				ContractAddress:         VaultAddress,
				AmountShares:            "50",
				PositionStartHeight:     1000,
				PositionEndHeight:       nil,
				IsTerminated:            nil,
				WithdrawReceiverAddress: nil,
				PositionIndexId:         101,
			},
		}

		var expectedUpdates []database.PublicPositionsUpdate = nil

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})

	t.Run("deposit, existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var receiverPosition = database.PublicPositionsSelect{
			Id:                      PositionId1,
			PositionIndexId:         0,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
		}
		var event = PositionEvent{
			EventName: "Transfer",
			EventData: map[string]interface{}{
				"from":  common.HexToAddress(ZeroAddress),
				"to":    common.HexToAddress(UserAddress1),
				"value": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, nil, &receiverPosition, maxPositionIndexId)

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                receiverPosition.Id,
				PositionEndHeight: toInt64Ptr(1999),
				IsTerminated:      toBoolPtr(false),
			},
		}
		var expectedInserts = []database.PublicPositionsInsert{
			{
				OwnerAddress:            UserAddress1,
				ContractAddress:         VaultAddress,
				AmountShares:            "150",
				PositionStartHeight:     2000,
				PositionEndHeight:       nil,
				IsTerminated:            nil,
				WithdrawReceiverAddress: nil,
				PositionIndexId:         101,
			},
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}

func TestProcessTransfer(t *testing.T) {
	t.Run("transfer, no existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Transfer",
			EventData: map[string]interface{}{
				"from":  common.HexToAddress(UserAddress1),
				"to":    common.HexToAddress(UserAddress2),
				"value": big.NewInt(100),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		var senderPosition = &database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
		}

		var receiverPosition *database.PublicPositionsSelect = nil
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PublicPositionsInsert{
			{
				OwnerAddress:            UserAddress2,
				ContractAddress:         VaultAddress,
				AmountShares:            "100",
				PositionStartHeight:     2000,
				PositionEndHeight:       nil,
				IsTerminated:            nil,
				WithdrawReceiverAddress: nil,
				PositionIndexId:         101,
			},
		}

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                senderPosition.Id,
				PositionEndHeight: toInt64Ptr(1999),
				IsTerminated:      toBoolPtr(true),
			},
		}

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})

	t.Run("full transfer, existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Transfer",
			EventData: map[string]interface{}{
				"from":  common.HexToAddress(UserAddress1),
				"to":    common.HexToAddress(UserAddress2),
				"value": big.NewInt(100),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		var senderPosition = &database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
			PositionIndexId:         0,
		}

		var receiverPosition = &database.PublicPositionsSelect{
			Id:                      PositionId2,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress2,
			ContractAddress:         VaultAddress,
			PositionIndexId:         1,
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PublicPositionsInsert{
			{
				OwnerAddress:            UserAddress2,
				ContractAddress:         VaultAddress,
				AmountShares:            "200",
				PositionStartHeight:     2000,
				PositionEndHeight:       nil,
				IsTerminated:            nil,
				WithdrawReceiverAddress: nil,
				PositionIndexId:         101,
			},
		}

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                receiverPosition.Id,
				PositionEndHeight: toInt64Ptr(1999),
				IsTerminated:      toBoolPtr(false),
			},
			{
				Id:                senderPosition.Id,
				PositionEndHeight: toInt64Ptr(1999),
				IsTerminated:      toBoolPtr(true),
			},
		}

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})

	t.Run("partial transfer, existing position", func(t *testing.T) {
		processor := &PositionProcessor{}
		var event = PositionEvent{
			EventName: "Transfer",
			EventData: map[string]interface{}{
				"from":  common.HexToAddress(UserAddress1),
				"to":    common.HexToAddress(UserAddress2),
				"value": big.NewInt(50),
			},
			Log: types.Log{
				Address:     common.HexToAddress(VaultAddress),
				BlockNumber: 2000,
			},
		}
		var senderPosition = &database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
			PositionIndexId:         0,
		}

		var receiverPosition = &database.PublicPositionsSelect{
			Id:                      PositionId2,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress2,
			ContractAddress:         VaultAddress,
			PositionIndexId:         1,
		}
		gotInserts, gotUpdates, err := processor.processPositionEvent(event, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PublicPositionsInsert{
			{
				OwnerAddress:            UserAddress2,
				ContractAddress:         VaultAddress,
				AmountShares:            "150",
				PositionStartHeight:     2000,
				PositionEndHeight:       nil,
				IsTerminated:            nil,
				WithdrawReceiverAddress: nil,
				PositionIndexId:         101,
			},
			{
				OwnerAddress:            UserAddress1,
				ContractAddress:         VaultAddress,
				AmountShares:            "50",
				PositionStartHeight:     2000,
				PositionEndHeight:       nil,
				IsTerminated:            nil,
				WithdrawReceiverAddress: nil,
				PositionIndexId:         102,
			},
		}

		var expectedUpdates = []database.PublicPositionsUpdate{
			{
				Id:                receiverPosition.Id,
				PositionEndHeight: toInt64Ptr(1999),
				IsTerminated:      toBoolPtr(false),
			},
			{
				Id:                senderPosition.Id,
				PositionEndHeight: toInt64Ptr(1999),
				IsTerminated:      toBoolPtr(false),
			},
		}

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})
}

func TestProcessWithdraw(t *testing.T) {

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

func TestUpdatePosition(t *testing.T) {
	t.Run("update position with addition", func(t *testing.T) {
		currentPosition := &database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
			PositionIndexId:         0,
		}

		insert, update := updatePosition(currentPosition, UserAddress1, "50", 2000, true, nil, toInt64Ptr(maxPositionIndexId))

		expectedInsert := &database.PublicPositionsInsert{
			OwnerAddress:        UserAddress1,
			ContractAddress:     VaultAddress,
			AmountShares:        "150",
			PositionStartHeight: 2000,
			PositionIndexId:     101,
		}

		expectedUpdate := &database.PublicPositionsUpdate{
			Id:                currentPosition.Id,
			PositionEndHeight: toInt64Ptr(1999),
			IsTerminated:      toBoolPtr(false),
		}

		assert.Equal(t, expectedInsert, insert)
		assert.Equal(t, expectedUpdate, update)
	})

	t.Run("update position with subtraction to zero", func(t *testing.T) {
		currentPosition := &database.PublicPositionsSelect{
			Id:                      PositionId1,
			AmountShares:            "100",
			PositionEndHeight:       nil,
			IsTerminated:            toBoolPtr(false),
			WithdrawReceiverAddress: nil,
			PositionStartHeight:     1000,
			OwnerAddress:            UserAddress1,
			ContractAddress:         VaultAddress,
		}

		insert, update := updatePosition(currentPosition, UserAddress1, "100", 2000, false, nil, toInt64Ptr(0))

		expectedUpdate := &database.PublicPositionsUpdate{
			Id:                currentPosition.Id,
			PositionEndHeight: toInt64Ptr(1999),
			IsTerminated:      toBoolPtr(true),
		}

		assert.Nil(t, insert)
		assert.Equal(t, expectedUpdate, update)
	})

}
