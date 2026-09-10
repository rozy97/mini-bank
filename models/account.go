package models

import "time"

type Account struct {
	ID        int64     `db:"id"`
	OwnerID   int64     `db:"owner_id"`
	Balance   int64     `db:"balance"`
	Currency  string    `db:"currency"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
