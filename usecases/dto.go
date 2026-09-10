package usecases

import "time"

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type RegisterOutput struct {
	UserID    int64
	Name      string
	Email     string
	AccountID int64
	Balance   int64
	Currency  string
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginOutput struct {
	UserID    int64
	Token     string
	ExpiresAt time.Time
}

type BalanceOutput struct {
	AccountID int64
	Balance   int64
	Currency  string
}

type TransferInput struct {
	FromUserID     int64
	ToAccountID    int64
	Amount         int64
	Description    string
	IdempotencyKey string
	RequestHash    string
}

type TransferOutput struct {
	TransferID    int64     `json:"transfer_id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}

type HistoryInput struct {
	UserID   int64
	Page     int
	PageSize int
}

type HistoryEntry struct {
	ID           int64
	TransferID   *int64
	Amount       int64
	BalanceAfter int64
	CreatedAt    time.Time
}

type HistoryOutput struct {
	Entries    []HistoryEntry
	Page       int
	PageSize   int
	TotalItems int64
	TotalPages int
}
