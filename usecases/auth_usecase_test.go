package usecases_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rozy97/mini-bank/pkg/password"
	"github.com/rozy97/mini-bank/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthUsecase() *usecases.AuthUsecase {
	return usecases.NewAuthUsecase(
		newFakeUserRepo(),
		newFakeAccountRepo(),
		fakeTxManager{},
		password.NewBcryptHasher(),
		fakeTokenManager{},
	)
}

func TestAuthUsecase_Register_PrepopulatesBalance(t *testing.T) {
	uc := newAuthUsecase()

	out, err := uc.Register(context.Background(), usecases.RegisterInput{
		Name:     "Jane Doe",
		Email:    "jane@example.com",
		Password: "supersecret123",
	})

	require.NoError(t, err)
	assert.Equal(t, "Jane Doe", out.Name)
	assert.Equal(t, "jane@example.com", out.Email)
	assert.Equal(t, int64(10_000_000), out.Balance)
	assert.Equal(t, "IDR", out.Currency)
	assert.NotZero(t, out.UserID)
	assert.NotZero(t, out.AccountID)
}

func TestAuthUsecase_Register_DuplicateEmail(t *testing.T) {
	uc := newAuthUsecase()
	ctx := context.Background()
	in := usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"}

	_, err := uc.Register(ctx, in)
	require.NoError(t, err)

	_, err = uc.Register(ctx, usecases.RegisterInput{Name: "Impostor", Email: "JANE@example.com", Password: "otherpassword"})
	assert.ErrorIs(t, err, usecases.ErrEmailAlreadyExists)
}

func TestAuthUsecase_Register_PasswordTooLongToHash(t *testing.T) {
	uc := newAuthUsecase()

	// bcrypt refuses passwords over 72 bytes; this exercises the hasher.Hash
	// error path without needing a fake hasher.
	_, err := uc.Register(context.Background(), usecases.RegisterInput{
		Name:     "Jane Doe",
		Email:    "jane@example.com",
		Password: strings.Repeat("a", 100),
	})
	assert.ErrorIs(t, err, usecases.ErrPasswordTooLong, "must map to the client-error sentinel, not surface as an opaque internal error")
}

func TestAuthUsecase_Register_GenericHashError(t *testing.T) {
	uc := usecases.NewAuthUsecase(newFakeUserRepo(), newFakeAccountRepo(), fakeTxManager{}, fakePasswordHasher{hashErr: errBoom}, fakeTokenManager{})

	_, err := uc.Register(context.Background(), usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrPasswordTooLong, "a non-bcrypt hasher error must not be mislabeled as password-too-long")
}

func TestAuthUsecase_Register_UserRepoGenericError(t *testing.T) {
	userRepo := newFakeUserRepo()
	userRepo.createErr = errBoom // not a *pgconn.PgError, so isUniqueViolation is false
	uc := usecases.NewAuthUsecase(userRepo, newFakeAccountRepo(), fakeTxManager{}, password.NewBcryptHasher(), fakeTokenManager{})

	_, err := uc.Register(context.Background(), usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrEmailAlreadyExists)
}

func TestAuthUsecase_Register_UserRepoDifferentConstraintViolation(t *testing.T) {
	userRepo := newFakeUserRepo()
	// A *pgconn.PgError whose code isn't 23505 (unique_violation) must not be
	// swallowed as ErrEmailAlreadyExists.
	userRepo.createErr = &pgconn.PgError{Code: "23503"}
	uc := usecases.NewAuthUsecase(userRepo, newFakeAccountRepo(), fakeTxManager{}, password.NewBcryptHasher(), fakeTokenManager{})

	_, err := uc.Register(context.Background(), usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, usecases.ErrEmailAlreadyExists)
}

func TestAuthUsecase_Register_AccountRepoError(t *testing.T) {
	accountRepo := newFakeAccountRepo()
	accountRepo.createErr = errBoom
	uc := usecases.NewAuthUsecase(newFakeUserRepo(), accountRepo, fakeTxManager{}, password.NewBcryptHasher(), fakeTokenManager{})

	_, err := uc.Register(context.Background(), usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	assert.ErrorIs(t, err, errBoom)
}

func TestAuthUsecase_Login_Success(t *testing.T) {
	uc := newAuthUsecase()
	ctx := context.Background()

	_, err := uc.Register(ctx, usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	require.NoError(t, err)

	out, err := uc.Login(ctx, usecases.LoginInput{Email: "jane@example.com", Password: "supersecret123"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.Token)
	assert.False(t, out.ExpiresAt.IsZero())
}

func TestAuthUsecase_Login_WrongPassword(t *testing.T) {
	uc := newAuthUsecase()
	ctx := context.Background()

	_, err := uc.Register(ctx, usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	require.NoError(t, err)

	_, err = uc.Login(ctx, usecases.LoginInput{Email: "jane@example.com", Password: "wrongpassword"})
	assert.ErrorIs(t, err, usecases.ErrInvalidCredentials)
}

func TestAuthUsecase_Login_UnknownEmail(t *testing.T) {
	uc := newAuthUsecase()

	_, err := uc.Login(context.Background(), usecases.LoginInput{Email: "ghost@example.com", Password: "whatever123"})
	assert.ErrorIs(t, err, usecases.ErrInvalidCredentials)
}

func TestAuthUsecase_Login_UserRepoGenericError(t *testing.T) {
	userRepo := newFakeUserRepo()
	userRepo.getByEmailErr = errBoom
	uc := usecases.NewAuthUsecase(userRepo, newFakeAccountRepo(), fakeTxManager{}, password.NewBcryptHasher(), fakeTokenManager{})

	_, err := uc.Login(context.Background(), usecases.LoginInput{Email: "jane@example.com", Password: "whatever123"})
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, usecases.ErrInvalidCredentials)
}

func TestAuthUsecase_Login_TokenGenerationError(t *testing.T) {
	userRepo := newFakeUserRepo()
	uc := usecases.NewAuthUsecase(userRepo, newFakeAccountRepo(), fakeTxManager{}, password.NewBcryptHasher(), fakeTokenManager{generateErr: errBoom})
	ctx := context.Background()

	_, err := uc.Register(ctx, usecases.RegisterInput{Name: "Jane Doe", Email: "jane@example.com", Password: "supersecret123"})
	require.NoError(t, err)

	_, err = uc.Login(ctx, usecases.LoginInput{Email: "jane@example.com", Password: "supersecret123"})
	assert.ErrorIs(t, err, errBoom)
}
