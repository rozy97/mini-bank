package usecases_test

import (
	"context"
	"testing"

	"github.com/rozy97/mini-bank/models"
	"github.com/rozy97/mini-bank/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transferTestSetup struct {
	uc          *usecases.TransferUsecase
	accountRepo *fakeAccountRepo
	fromUserID  int64
	fromAccount int64
	toAccount   int64
}

func newTransferTestSetup(t *testing.T) transferTestSetup {
	t.Helper()
	ctx := context.Background()

	accountRepo := newFakeAccountRepo()
	uc := usecases.NewTransferUsecase(
		accountRepo,
		newFakeTransferRepo(),
		newFakeEntryRepo(),
		newFakeIdempotencyRepo(),
		fakeTxManager{},
	)

	from := &models.Account{OwnerID: 1, Balance: 10_000_000, Currency: "IDR"}
	require.NoError(t, accountRepo.Create(ctx, from))
	to := &models.Account{OwnerID: 2, Balance: 10_000_000, Currency: "IDR"}
	require.NoError(t, accountRepo.Create(ctx, to))

	return transferTestSetup{uc: uc, accountRepo: accountRepo, fromUserID: 1, fromAccount: from.ID, toAccount: to.ID}
}

func TestTransferUsecase_Success(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()

	out, err := s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000_000,
		Description:    "rent",
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1_000_000), out.Amount)

	from, err := s.accountRepo.GetByOwnerID(ctx, s.fromUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(9_000_000), from.Balance)

	to, err := s.accountRepo.GetForUpdate(ctx, s.toAccount)
	require.NoError(t, err)
	assert.Equal(t, int64(11_000_000), to.Balance)
}

func TestTransferUsecase_InsufficientBalance(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()

	_, err := s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         50_000_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrInsufficientBalance)

	from, err := s.accountRepo.GetByOwnerID(ctx, s.fromUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(10_000_000), from.Balance, "balance must be untouched on failure")
}

func TestTransferUsecase_SameAccount(t *testing.T) {
	s := newTransferTestSetup(t)

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.fromAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrSameAccountTransfer)
}

func TestTransferUsecase_InvalidAmount(t *testing.T) {
	s := newTransferTestSetup(t)

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         0,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrInvalidAmount)
}

func TestTransferUsecase_MissingIdempotencyKey(t *testing.T) {
	s := newTransferTestSetup(t)

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:  s.fromUserID,
		ToAccountID: s.toAccount,
		Amount:      1_000,
		RequestHash: "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrIdempotencyKeyRequired)
}

func TestTransferUsecase_IdempotentRetry_ReturnsSameResult(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()
	in := usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	}

	first, err := s.uc.Transfer(ctx, in)
	require.NoError(t, err)

	second, err := s.uc.Transfer(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, first.TransferID, second.TransferID)

	from, err := s.accountRepo.GetByOwnerID(ctx, s.fromUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(9_000_000), from.Balance, "the transfer must not be applied twice")
}

func TestTransferUsecase_IdempotencyKeyReusedWithDifferentPayload(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()

	_, err := s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	require.NoError(t, err)

	_, err = s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         2_000_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-2",
	})
	assert.ErrorIs(t, err, usecases.ErrIdempotencyKeyConflict)
}
