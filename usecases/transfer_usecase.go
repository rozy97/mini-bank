package usecases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rozy97/mini-bank/models"
)

// transferSuccessStatus is stored alongside the cached response body; it is
// a plain int (not an http.Status* constant) so this layer stays free of
// any transport-layer dependency.
const transferSuccessStatus = 201

// maxIdempotencyKeyLength matches idempotency_keys.idempotency_key's
// VARCHAR(255) column (migrations/001_initial_schema.sql). Rejecting an
// over-length key here, before it ever reaches the database, turns what
// would otherwise be an opaque 500 (a Postgres "value too long" error) into
// a clear 400 — this is a client mistake, not a server fault.
const maxIdempotencyKeyLength = 255

type TransferUsecase struct {
	accountRepo     AccountRepository
	transferRepo    TransferRepository
	entryRepo       EntryRepository
	idempotencyRepo IdempotencyRepository
	txManager       TxManager
}

func NewTransferUsecase(
	accountRepo AccountRepository,
	transferRepo TransferRepository,
	entryRepo EntryRepository,
	idempotencyRepo IdempotencyRepository,
	txManager TxManager,
) *TransferUsecase {
	return &TransferUsecase{
		accountRepo:     accountRepo,
		transferRepo:    transferRepo,
		entryRepo:       entryRepo,
		idempotencyRepo: idempotencyRepo,
		txManager:       txManager,
	}
}

// Transfer moves funds from the caller's account to another account. It is
// idempotent on (FromUserID, IdempotencyKey): retrying the exact same
// request returns the original response instead of transferring twice, and
// reusing the key with a different payload is rejected.
func (u *TransferUsecase) Transfer(ctx context.Context, in TransferInput) (*TransferOutput, error) {
	if in.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if in.IdempotencyKey == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	if len(in.IdempotencyKey) > maxIdempotencyKeyLength {
		return nil, ErrIdempotencyKeyTooLong
	}

	var output *TransferOutput

	err := u.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		record, created, err := u.idempotencyRepo.FindOrCreate(ctx, in.FromUserID, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}

		if !created {
			if record.RequestHash != in.RequestHash {
				return ErrIdempotencyKeyConflict
			}
			if record.ResponseBody == nil {
				return ErrIdempotencyKeyInProgress
			}
			var cached TransferOutput
			if err := json.Unmarshal(record.ResponseBody, &cached); err != nil {
				return fmt.Errorf("unmarshal cached transfer response: %w", err)
			}
			output = &cached
			return nil
		}

		fromAccount, err := u.accountRepo.GetByOwnerID(ctx, in.FromUserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrAccountNotFound
			}
			return err
		}
		if fromAccount.ID == in.ToAccountID {
			return ErrSameAccountTransfer
		}

		lockedFrom, lockedTo, err := u.lockAccountsInOrder(ctx, fromAccount.ID, in.ToAccountID)
		if err != nil {
			return err
		}

		if lockedFrom.Balance < in.Amount {
			return ErrInsufficientBalance
		}

		newFromBalance := lockedFrom.Balance - in.Amount
		newToBalance := lockedTo.Balance + in.Amount

		if err := u.accountRepo.UpdateBalance(ctx, lockedFrom.ID, newFromBalance); err != nil {
			return err
		}
		if err := u.accountRepo.UpdateBalance(ctx, lockedTo.ID, newToBalance); err != nil {
			return err
		}

		transfer := &models.Transfer{
			FromAccountID: lockedFrom.ID,
			ToAccountID:   lockedTo.ID,
			Amount:        in.Amount,
			Description:   in.Description,
		}
		if err := u.transferRepo.Create(ctx, transfer); err != nil {
			return err
		}

		debit := &models.Entry{AccountID: lockedFrom.ID, TransferID: &transfer.ID, Amount: -in.Amount, BalanceAfter: newFromBalance}
		credit := &models.Entry{AccountID: lockedTo.ID, TransferID: &transfer.ID, Amount: in.Amount, BalanceAfter: newToBalance}
		if err := u.entryRepo.Create(ctx, debit); err != nil {
			return err
		}
		if err := u.entryRepo.Create(ctx, credit); err != nil {
			return err
		}

		result := TransferOutput{
			TransferID:    transfer.ID,
			FromAccountID: lockedFrom.ID,
			ToAccountID:   lockedTo.ID,
			Amount:        in.Amount,
			Description:   in.Description,
			CreatedAt:     transfer.CreatedAt,
		}
		body, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal transfer response: %w", err)
		}
		if err := u.idempotencyRepo.Complete(ctx, record.ID, transferSuccessStatus, body); err != nil {
			return err
		}

		output = &result
		return nil
	})
	if err != nil {
		return nil, err
	}

	return output, nil
}

// lockAccountsInOrder takes row locks (SELECT ... FOR UPDATE) on both
// accounts in ascending ID order, regardless of which one is the sender.
// Two concurrent transfers between the same pair of accounts then always
// request locks in the same order, which rules out a deadlock.
func (u *TransferUsecase) lockAccountsInOrder(ctx context.Context, fromID, toID int64) (from, to *models.Account, err error) {
	notFound := func(err error) error {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccountNotFound
		}
		return err
	}

	if fromID < toID {
		if from, err = u.accountRepo.GetForUpdate(ctx, fromID); err != nil {
			return nil, nil, notFound(err)
		}
		if to, err = u.accountRepo.GetForUpdate(ctx, toID); err != nil {
			return nil, nil, notFound(err)
		}
		return from, to, nil
	}

	if to, err = u.accountRepo.GetForUpdate(ctx, toID); err != nil {
		return nil, nil, notFound(err)
	}
	if from, err = u.accountRepo.GetForUpdate(ctx, fromID); err != nil {
		return nil, nil, notFound(err)
	}
	return from, to, nil
}
