package usecases_test

import (
	"context"
	"testing"

	"github.com/rozy97/mini-bank/models"
	"github.com/rozy97/mini-bank/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountUsecase_GetBalance(t *testing.T) {
	ctx := context.Background()
	accountRepo := newFakeAccountRepo()
	uc := usecases.NewAccountUsecase(accountRepo, newFakeEntryRepo())

	acc := &models.Account{OwnerID: 1, Balance: 10_000_000, Currency: "IDR"}
	require.NoError(t, accountRepo.Create(ctx, acc))

	out, err := uc.GetBalance(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(10_000_000), out.Balance)
	assert.Equal(t, "IDR", out.Currency)
}

func TestAccountUsecase_GetBalance_NotFound(t *testing.T) {
	uc := usecases.NewAccountUsecase(newFakeAccountRepo(), newFakeEntryRepo())

	_, err := uc.GetBalance(context.Background(), 999)
	assert.ErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestAccountUsecase_GetBalance_RepoError(t *testing.T) {
	accountRepo := newFakeAccountRepo()
	accountRepo.getByOwnerErr = errBoom
	uc := usecases.NewAccountUsecase(accountRepo, newFakeEntryRepo())

	_, err := uc.GetBalance(context.Background(), 1)
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestAccountUsecase_GetHistory_NotFound(t *testing.T) {
	uc := usecases.NewAccountUsecase(newFakeAccountRepo(), newFakeEntryRepo())

	_, err := uc.GetHistory(context.Background(), usecases.HistoryInput{UserID: 999})
	assert.ErrorIs(t, err, usecases.ErrAccountNotFound)
}

func TestAccountUsecase_GetHistory_AccountRepoError(t *testing.T) {
	accountRepo := newFakeAccountRepo()
	accountRepo.getByOwnerErr = errBoom
	uc := usecases.NewAccountUsecase(accountRepo, newFakeEntryRepo())

	_, err := uc.GetHistory(context.Background(), usecases.HistoryInput{UserID: 1})
	assert.ErrorIs(t, err, errBoom)
}

func TestAccountUsecase_GetHistory_CountError(t *testing.T) {
	ctx := context.Background()
	accountRepo := newFakeAccountRepo()
	entryRepo := newFakeEntryRepo()
	uc := usecases.NewAccountUsecase(accountRepo, entryRepo)

	require.NoError(t, accountRepo.Create(ctx, &models.Account{OwnerID: 1}))
	entryRepo.countErr = errBoom

	_, err := uc.GetHistory(ctx, usecases.HistoryInput{UserID: 1})
	assert.ErrorIs(t, err, errBoom)
}

func TestAccountUsecase_GetHistory_ListError(t *testing.T) {
	ctx := context.Background()
	accountRepo := newFakeAccountRepo()
	entryRepo := newFakeEntryRepo()
	uc := usecases.NewAccountUsecase(accountRepo, entryRepo)

	require.NoError(t, accountRepo.Create(ctx, &models.Account{OwnerID: 1}))
	entryRepo.listErr = errBoom

	_, err := uc.GetHistory(ctx, usecases.HistoryInput{UserID: 1})
	assert.ErrorIs(t, err, errBoom)
}

func TestAccountUsecase_GetHistory_DefaultsAndClampsPaging(t *testing.T) {
	ctx := context.Background()
	accountRepo := newFakeAccountRepo()
	entryRepo := newFakeEntryRepo()
	uc := usecases.NewAccountUsecase(accountRepo, entryRepo)

	require.NoError(t, accountRepo.Create(ctx, &models.Account{OwnerID: 1}))

	// Page/PageSize <= 0 fall back to defaults; PageSize above the max is clamped.
	out, err := uc.GetHistory(ctx, usecases.HistoryInput{UserID: 1, Page: 0, PageSize: 0})
	require.NoError(t, err)
	assert.Equal(t, 1, out.Page)
	assert.Equal(t, 20, out.PageSize)

	out, err = uc.GetHistory(ctx, usecases.HistoryInput{UserID: 1, Page: -1, PageSize: 1000})
	require.NoError(t, err)
	assert.Equal(t, 1, out.Page)
	assert.Equal(t, 100, out.PageSize)
}

func TestAccountUsecase_GetHistory_Paginates(t *testing.T) {
	ctx := context.Background()
	accountRepo := newFakeAccountRepo()
	entryRepo := newFakeEntryRepo()
	uc := usecases.NewAccountUsecase(accountRepo, entryRepo)

	acc := &models.Account{OwnerID: 1, Balance: 10_000_000, Currency: "IDR"}
	require.NoError(t, accountRepo.Create(ctx, acc))

	for i := 0; i < 5; i++ {
		require.NoError(t, entryRepo.Create(ctx, &models.Entry{
			AccountID:    acc.ID,
			Amount:       -1000,
			BalanceAfter: 10_000_000 - int64(i+1)*1000,
		}))
	}

	page1, err := uc.GetHistory(ctx, usecases.HistoryInput{UserID: 1, Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Len(t, page1.Entries, 2)
	assert.Equal(t, int64(5), page1.TotalItems)
	assert.Equal(t, 3, page1.TotalPages)
	// Newest first: the 5th entry created has the lowest BalanceAfter.
	assert.Equal(t, int64(10_000_000-5*1000), page1.Entries[0].BalanceAfter)

	page3, err := uc.GetHistory(ctx, usecases.HistoryInput{UserID: 1, Page: 3, PageSize: 2})
	require.NoError(t, err)
	assert.Len(t, page3.Entries, 1)
}
