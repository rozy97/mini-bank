package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rozy97/mini-bank/models"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts u and populates its ID/CreatedAt/UpdatedAt. Callers should
// check the returned error with errors.As for a *pgconn.PgError with code
// 23505 to detect a duplicate email.
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	ex := executorFromContext(ctx, r.db)
	query := `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`
	if err := ex.GetContext(ctx, u, query, u.Name, u.Email, u.PasswordHash); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetByEmail returns sql.ErrNoRows (wrapped) when no user matches.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	ex := executorFromContext(ctx, r.db)
	var u models.User
	query := `
		SELECT id, name, email, password_hash, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	if err := ex.GetContext(ctx, &u, query, email); err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}
