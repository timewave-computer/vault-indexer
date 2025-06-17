package transformer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/timewave/vault-indexer/go-indexer/database"
	"github.com/timewave/vault-indexer/go-indexer/logger"
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

var processor = &PositionTransformer{
	logger: logger.NewLogger("Transformer:Position"),
}

func TestProcessDeposit(t *testing.T) {

	t.Run("deposit, no existing position", func(t *testing.T) {

		var args = ProcessPosition{
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "50",
			BlockNumber:     1000,
		}

		var senderPosition *database.PublicPositionsSelect = nil
		gotInserts, gotUpdates, err := processor.ComputeDeposit(args, senderPosition, maxPositionIndexId)

		var expectedInserts = []database.PositionInsert{
			{
				OwnerAddress:        UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
				PositionStartHeight: 1000,
				PositionIndexId:     101,
			},
		}

		var expectedUpdates []database.PositionUpdate = nil

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})

	t.Run("deposit, existing position", func(t *testing.T) {

		var senderPosition = database.PublicPositionsSelect{
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
		var args = ProcessPosition{
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "50",
			BlockNumber:     2000,
		}

		gotInserts, gotUpdates, err := processor.ComputeDeposit(args, &senderPosition, maxPositionIndexId)

		var expectedUpdates = []database.PositionUpdate{
			{
				Id:                      senderPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            false,
				WithdrawReceiverAddress: nil,
			},
		}
		var expectedInserts = []database.PositionInsert{
			{
				OwnerAddress:        UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "150",
				PositionStartHeight: 2000,
				PositionIndexId:     101,
			},
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})
}

func TestProcessTransfer(t *testing.T) {
	t.Run("transfer, no existing position", func(t *testing.T) {
		var args = ProcessPosition{
			ReceiverAddress: UserAddress2,
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "100",
			BlockNumber:     2000,
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
		gotInserts, gotUpdates, err := processor.ComputeTransfer(args, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PositionInsert{
			{
				OwnerAddress:        UserAddress2,
				ContractAddress:     VaultAddress,
				AmountShares:        "100",
				PositionStartHeight: 2000,
				PositionIndexId:     101,
			},
		}

		var expectedUpdates = []database.PositionUpdate{
			{
				Id:                      senderPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            true,
				WithdrawReceiverAddress: nil,
			},
		}

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})

	t.Run("full transfer, existing position", func(t *testing.T) {
		var args = ProcessPosition{
			ReceiverAddress: UserAddress2,
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "100",
			BlockNumber:     2000,
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
		gotInserts, gotUpdates, err := processor.ComputeTransfer(args, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PositionInsert{
			{
				OwnerAddress:        UserAddress2,
				ContractAddress:     VaultAddress,
				AmountShares:        "200",
				PositionStartHeight: 2000,
				PositionIndexId:     101,
			},
		}

		var expectedUpdates = []database.PositionUpdate{
			{
				Id:                      receiverPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            false,
				WithdrawReceiverAddress: nil,
			},
			{
				Id:                      senderPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            true,
				WithdrawReceiverAddress: nil,
			},
		}

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})

	t.Run("partial transfer, existing position", func(t *testing.T) {
		var args = ProcessPosition{
			ReceiverAddress: UserAddress2,
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "50",
			BlockNumber:     2000,
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
		gotInserts, gotUpdates, err := processor.ComputeTransfer(args, senderPosition, receiverPosition, maxPositionIndexId)

		var expectedInserts = []database.PositionInsert{
			{
				OwnerAddress:        UserAddress2,
				ContractAddress:     VaultAddress,
				AmountShares:        "150",
				PositionStartHeight: 2000,
				PositionIndexId:     101,
			},
			{
				OwnerAddress:        UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
				PositionStartHeight: 2000,
				PositionIndexId:     102,
			},
		}

		var expectedUpdates = []database.PositionUpdate{
			{
				Id:                      receiverPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            false,
				WithdrawReceiverAddress: nil,
			},
			{
				Id:                      senderPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            false,
				WithdrawReceiverAddress: nil,
			},
		}

		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
		assert.NoError(t, err)

	})
}

func TestProcessWithdraw(t *testing.T) {

	t.Run("partial withdraw", func(t *testing.T) {
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
		var args = ProcessPosition{
			ReceiverAddress: NeutronAddress1,
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "50",
			BlockNumber:     2000,
		}
		gotInserts, gotUpdates, err := processor.ComputeWithdraw(args, &currentPosition, maxPositionIndexId)

		var expectedUpdates = []database.PositionUpdate{
			{
				Id:                      currentPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            false,
				WithdrawReceiverAddress: toStringPtr(NeutronAddress1),
			},
		}
		var expectedInserts = []database.PositionInsert{
			{
				OwnerAddress:        UserAddress1,
				ContractAddress:     VaultAddress,
				AmountShares:        "50",
				PositionStartHeight: 2000,
				PositionIndexId:     101,
			},
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})

	t.Run("full withdraw", func(t *testing.T) {
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
		var args = ProcessPosition{
			ReceiverAddress: NeutronAddress1,
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "100",
			BlockNumber:     2000,
		}
		gotInserts, gotUpdates, err := processor.ComputeWithdraw(args, &senderPosition, maxPositionIndexId)

		var expectedInserts = []database.PositionInsert(nil)
		var expectedUpdates = []database.PositionUpdate{
			{
				Id:                      senderPosition.Id,
				PositionEndHeight:       1999,
				IsTerminated:            true,
				WithdrawReceiverAddress: toStringPtr(NeutronAddress1),
			},
		}

		assert.NoError(t, err)
		assert.Equal(t, expectedInserts, gotInserts)
		assert.Equal(t, expectedUpdates, gotUpdates)
	})

	t.Run("no existing position", func(t *testing.T) {
		var args = ProcessPosition{
			ReceiverAddress: NeutronAddress1,
			SenderAddress:   UserAddress1,
			ContractAddress: VaultAddress,
			AmountShares:    "100",
			BlockNumber:     2000,
		}
		gotInserts, gotUpdates, err := processor.ComputeWithdraw(args, nil, maxPositionIndexId)
		assert.NoError(t, err)
		assert.Equal(t, []database.PositionInsert(nil), gotInserts)
		assert.Equal(t, []database.PositionUpdate(nil), gotUpdates)
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

		insert, update, err := processor.UpdatePosition(UpdatePositionInput{
			CurrentPosition: currentPosition,
			Address:         UserAddress1,
			AmountShares:    "50",
			BlockNumber:     2000,
			IsAddition:      true,
		}, toInt64Ptr(maxPositionIndexId))

		expectedInsert := &database.PositionInsert{
			OwnerAddress:        UserAddress1,
			ContractAddress:     VaultAddress,
			AmountShares:        "150",
			PositionStartHeight: 2000,
			PositionIndexId:     101,
		}

		expectedUpdate := &database.PositionUpdate{
			Id:                      currentPosition.Id,
			PositionEndHeight:       1999,
			IsTerminated:            false,
			WithdrawReceiverAddress: nil,
		}

		assert.Equal(t, expectedInsert, insert)
		assert.Equal(t, expectedUpdate, update)
		assert.NoError(t, err)
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

		insert, update, err := processor.UpdatePosition(UpdatePositionInput{
			CurrentPosition: currentPosition,
			Address:         UserAddress1,
			AmountShares:    "100",
			BlockNumber:     2000,
			IsAddition:      false,
		}, toInt64Ptr(0))

		expectedUpdate := &database.PositionUpdate{
			Id:                      currentPosition.Id,
			PositionEndHeight:       1999,
			IsTerminated:            true,
			WithdrawReceiverAddress: nil,
		}

		assert.Nil(t, insert)
		assert.Equal(t, expectedUpdate, update)
		assert.NoError(t, err)
	})

}
