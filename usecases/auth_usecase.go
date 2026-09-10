package usecases

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/rozy97/mini-bank/models"
)

const (
	initialBalance  = 10_000_000
	defaultCurrency = "IDR"
)

type AuthUsecase struct {
	userRepo    UserRepository
	accountRepo AccountRepository
	txManager   TxManager
	hasher      PasswordHasher
	tokens      TokenManager
}

func NewAuthUsecase(
	userRepo UserRepository,
	accountRepo AccountRepository,
	txManager TxManager,
	hasher PasswordHasher,
	tokens TokenManager,
) *AuthUsecase {
	return &AuthUsecase{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		txManager:   txManager,
		hasher:      hasher,
		tokens:      tokens,
	}
}

// Register creates a user and, in the same transaction, an account
// prepopulated with the initial balance.
func (u *AuthUsecase) Register(ctx context.Context, in RegisterInput) (*RegisterOutput, error) {
	hash, err := u.hasher.Hash(in.Password)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return nil, ErrPasswordTooLong
		}
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Name:         strings.TrimSpace(in.Name),
		Email:        strings.ToLower(strings.TrimSpace(in.Email)),
		PasswordHash: hash,
	}
	account := &models.Account{
		Balance:  initialBalance,
		Currency: defaultCurrency,
	}

	err = u.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := u.userRepo.Create(ctx, user); err != nil {
			if isUniqueViolation(err) {
				return ErrEmailAlreadyExists
			}
			return err
		}

		account.OwnerID = user.ID
		return u.accountRepo.Create(ctx, account)
	})
	if err != nil {
		return nil, err
	}

	return &RegisterOutput{
		UserID:    user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AccountID: account.ID,
		Balance:   account.Balance,
		Currency:  account.Currency,
	}, nil
}

func (u *AuthUsecase) Login(ctx context.Context, in LoginInput) (*LoginOutput, error) {
	user, err := u.userRepo.GetByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := u.hasher.Compare(user.PasswordHash, in.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := u.tokens.Generate(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &LoginOutput{UserID: user.ID, Token: token, ExpiresAt: expiresAt}, nil
}
