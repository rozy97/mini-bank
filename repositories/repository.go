package repositories

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rozy97/mini-bank/models"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user *models.User
	query := `
	SELECT
		id,
		name,
		email
	FROM
		users
	WHERE
		id = $1
	`
	err := r.db.GetContext(ctx, &user, query, id)

	return user, err
}
