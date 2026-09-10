package usecases_test

import (
	"context"
	"testing"

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
