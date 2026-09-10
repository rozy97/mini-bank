package usecases

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type AccountUsecase struct {
	accountRepo AccountRepository
	entryRepo   EntryRepository
}

func NewAccountUsecase(accountRepo AccountRepository, entryRepo EntryRepository) *AccountUsecase {
	return &AccountUsecase{accountRepo: accountRepo, entryRepo: entryRepo}
}

func (u *AccountUsecase) GetBalance(ctx context.Context, userID int64) (*BalanceOutput, error) {
	account, err := u.accountRepo.GetByOwnerID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	return &BalanceOutput{
		AccountID: account.ID,
		Balance:   account.Balance,
		Currency:  account.Currency,
	}, nil
}

// GetHistory returns the authenticated user's ledger entries, newest first.
func (u *AccountUsecase) GetHistory(ctx context.Context, in HistoryInput) (*HistoryOutput, error) {
	account, err := u.accountRepo.GetByOwnerID(ctx, in.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize

	total, err := u.entryRepo.CountByAccountID(ctx, account.ID)
	if err != nil {
		return nil, fmt.Errorf("count entries: %w", err)
	}

	entries, err := u.entryRepo.ListByAccountID(ctx, account.ID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	out := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		out[i] = HistoryEntry{
			ID:           e.ID,
			TransferID:   e.TransferID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			CreatedAt:    e.CreatedAt,
		}
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &HistoryOutput{
		Entries:    out,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}
