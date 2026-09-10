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
	uc              *usecases.TransferUsecase
	accountRepo     *fakeAccountRepo
	transferRepo    *fakeTransferRepo
	entryRepo       *fakeEntryRepo
	idempotencyRepo *fakeIdempotencyRepo
	fromUserID      int64
	toUserID        int64
	fromAccount     int64
	toAccount       int64
}

func newTransferTestSetup(t *testing.T) transferTestSetup {
	t.Helper()
	ctx := context.Background()

	accountRepo := newFakeAccountRepo()
	transferRepo := newFakeTransferRepo()
	entryRepo := newFakeEntryRepo()
	idempotencyRepo := newFakeIdempotencyRepo()
	uc := usecases.NewTransferUsecase(accountRepo, transferRepo, entryRepo, idempotencyRepo, fakeTxManager{})

	from := &models.Account{OwnerID: 1, Balance: 10_000_000, Currency: "IDR"}
	require.NoError(t, accountRepo.Create(ctx, from))
	to := &models.Account{OwnerID: 2, Balance: 10_000_000, Currency: "IDR"}
	require.NoError(t, accountRepo.Create(ctx, to))

	return transferTestSetup{
		uc:              uc,
		accountRepo:     accountRepo,
		transferRepo:    transferRepo,
		entryRepo:       entryRepo,
		idempotencyRepo: idempotencyRepo,
		fromUserID:      1,
		toUserID:        2,
		fromAccount:     from.ID,
		toAccount:       to.ID,
	}
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

func TestTransferUsecase_ToAccountNotFound(t *testing.T) {
	s := newTransferTestSetup(t)

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    999, // fromAccount.ID(1) < 999, so this exercises the forward lock order
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestTransferUsecase_FromAccountNotFound(t *testing.T) {
	s := newTransferTestSetup(t)

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     999, // no account owned by this user
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestTransferUsecase_FromAccountRepoGenericError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.accountRepo.getByOwnerErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestTransferUsecase_FromAccountLockError(t *testing.T) {
	s := newTransferTestSetup(t)
	// fromAccount.ID(1) < toAccount.ID(2): forward lock order, error on the
	// first GetForUpdate call.
	s.accountRepo.getForUpdateErrByID = map[int64]error{s.fromAccount: errBoom}

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestTransferUsecase_ReverseLockOrder_Success(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()

	// toAccount.ID(2) > fromAccount.ID(1) normally; transferring the other
	// way round (from the higher-ID account to the lower-ID one) exercises
	// the reverse lock order branch.
	out, err := s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.toUserID,
		ToAccountID:    s.fromAccount,
		Amount:         1_000_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	require.NoError(t, err)
	assert.Equal(t, s.toAccount, out.FromAccountID)
	assert.Equal(t, s.fromAccount, out.ToAccountID)

	from, err := s.accountRepo.GetByOwnerID(ctx, s.toUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(9_000_000), from.Balance)
}

func TestTransferUsecase_ReverseLockOrder_ToNotFound(t *testing.T) {
	s := newTransferTestSetup(t)

	// toUserID owns account 2; toAccountID 0 was never assigned. 0 < 2 takes
	// the reverse lock order branch and fails on its first GetForUpdate call.
	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.toUserID,
		ToAccountID:    0,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestTransferUsecase_ReverseLockOrder_FromError(t *testing.T) {
	s := newTransferTestSetup(t)
	// Sender is toUserID's account (id 2), recipient is account 1: 2 > 1
	// takes the reverse lock order branch; erroring account 2's lock
	// exercises that branch's second GetForUpdate call (the sender's).
	s.accountRepo.getForUpdateErrByID = map[int64]error{s.toAccount: errBoom}

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.toUserID,
		ToAccountID:    s.fromAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestTransferUsecase_UpdateBalance_FromAccountError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.accountRepo.failUpdateBalanceOnCall = 1
	s.accountRepo.updateBalanceErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_UpdateBalance_ToAccountError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.accountRepo.failUpdateBalanceOnCall = 2
	s.accountRepo.updateBalanceErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_TransferRepoCreateError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.transferRepo.createErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_EntryRepoCreate_DebitError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.entryRepo.failCreateOnCall = 1
	s.entryRepo.createErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_EntryRepoCreate_CreditError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.entryRepo.failCreateOnCall = 2
	s.entryRepo.createErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_IdempotencyRepoFindOrCreateError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.idempotencyRepo.findOrCreateErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_IdempotencyRepoCompleteError(t *testing.T) {
	s := newTransferTestSetup(t)
	s.idempotencyRepo.completeErr = errBoom

	_, err := s.uc.Transfer(context.Background(), usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestTransferUsecase_IdempotencyKeyInProgress(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()

	// Seed a record via FindOrCreate without ever calling Complete, as if a
	// prior attempt crashed mid-transfer.
	_, created, err := s.idempotencyRepo.FindOrCreate(ctx, s.fromUserID, "key-1", "hash-1")
	require.NoError(t, err)
	require.True(t, created)

	_, err = s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	assert.ErrorIs(t, err, usecases.ErrIdempotencyKeyInProgress)
}

func TestTransferUsecase_CorruptedCachedResponse(t *testing.T) {
	s := newTransferTestSetup(t)
	ctx := context.Background()

	rec, created, err := s.idempotencyRepo.FindOrCreate(ctx, s.fromUserID, "key-1", "hash-1")
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, s.idempotencyRepo.Complete(ctx, rec.ID, 201, []byte("not valid json")))

	_, err = s.uc.Transfer(ctx, usecases.TransferInput{
		FromUserID:     s.fromUserID,
		ToAccountID:    s.toAccount,
		Amount:         1_000,
		IdempotencyKey: "key-1",
		RequestHash:    "hash-1",
	})
	require.Error(t, err)
}
