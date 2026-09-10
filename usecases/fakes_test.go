package usecases_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rozy97/mini-bank/models"
)

// errNotFound stands in for the wrapped sql.ErrNoRows that real repositories
// return, since usecases detect "not found" via errors.Is(err, sql.ErrNoRows).
var errNotFound = sql.ErrNoRows

// errBoom is an arbitrary, non-sentinel failure used to exercise the plain
// error-passthrough branches (as opposed to the "not found" branches).
var errBoom = errors.New("boom")

// fakeTxManager just runs fn against the same context: the usecase tests
// don't need real transactional isolation, only the composition behavior.
type fakeTxManager struct{}

func (fakeTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeUserRepo struct {
	mu            sync.Mutex
	byEmail       map[string]*models.User
	nextID        int64
	createErr     error // returned as-is, in place of the normal duplicate-email check
	getByEmailErr error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]*models.User{}}
}

func (f *fakeUserRepo) Create(ctx context.Context, u *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return f.createErr
	}

	key := strings.ToLower(u.Email)
	if _, exists := f.byEmail[key]; exists {
		return &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
	}

	f.nextID++
	u.ID = f.nextID
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()

	cp := *u
	f.byEmail[key] = &cp
	return nil
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.getByEmailErr != nil {
		return nil, f.getByEmailErr
	}

	u, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, errNotFound
	}
	cp := *u
	return &cp, nil
}

type fakeAccountRepo struct {
	mu      sync.Mutex
	byID    map[int64]*models.Account
	byOwner map[int64]int64 // ownerID -> accountID
	nextID  int64

	createErr           error
	getByOwnerErr       error
	getForUpdateErrByID map[int64]error // per-account-ID override, checked before the normal lookup

	updateBalanceCalls      int
	failUpdateBalanceOnCall int // 1-indexed; 0 means never fail
	updateBalanceErr        error
}

func newFakeAccountRepo() *fakeAccountRepo {
	return &fakeAccountRepo{byID: map[int64]*models.Account{}, byOwner: map[int64]int64{}}
}

func (f *fakeAccountRepo) Create(ctx context.Context, a *models.Account) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return f.createErr
	}

	f.nextID++
	a.ID = f.nextID
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()

	cp := *a
	f.byID[a.ID] = &cp
	f.byOwner[a.OwnerID] = a.ID
	return nil
}

func (f *fakeAccountRepo) GetByOwnerID(ctx context.Context, ownerID int64) (*models.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.getByOwnerErr != nil {
		return nil, f.getByOwnerErr
	}

	id, ok := f.byOwner[ownerID]
	if !ok {
		return nil, errNotFound
	}
	cp := *f.byID[id]
	return &cp, nil
}

func (f *fakeAccountRepo) GetForUpdate(ctx context.Context, id int64) (*models.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err, ok := f.getForUpdateErrByID[id]; ok {
		return nil, err
	}

	a, ok := f.byID[id]
	if !ok {
		return nil, errNotFound
	}
	cp := *a
	return &cp, nil
}

func (f *fakeAccountRepo) UpdateBalance(ctx context.Context, id int64, balance int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.updateBalanceCalls++
	if f.failUpdateBalanceOnCall != 0 && f.updateBalanceCalls == f.failUpdateBalanceOnCall {
		return f.updateBalanceErr
	}

	a, ok := f.byID[id]
	if !ok {
		return errNotFound
	}
	a.Balance = balance
	return nil
}

type fakeTransferRepo struct {
	mu        sync.Mutex
	nextID    int64
	all       []models.Transfer
	createErr error
}

func newFakeTransferRepo() *fakeTransferRepo { return &fakeTransferRepo{} }

func (f *fakeTransferRepo) Create(ctx context.Context, t *models.Transfer) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return f.createErr
	}

	f.nextID++
	t.ID = f.nextID
	t.CreatedAt = time.Now()
	f.all = append(f.all, *t)
	return nil
}

type fakeEntryRepo struct {
	mu     sync.Mutex
	nextID int64
	all    []models.Entry

	createCalls      int
	failCreateOnCall int // 1-indexed; 0 means never fail
	createErr        error

	listErr  error
	countErr error
}

func newFakeEntryRepo() *fakeEntryRepo { return &fakeEntryRepo{} }

func (f *fakeEntryRepo) Create(ctx context.Context, e *models.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.createCalls++
	if f.failCreateOnCall != 0 && f.createCalls == f.failCreateOnCall {
		return f.createErr
	}

	f.nextID++
	e.ID = f.nextID
	e.CreatedAt = time.Now()
	f.all = append(f.all, *e)
	return nil
}

func (f *fakeEntryRepo) ListByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]models.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listErr != nil {
		return nil, f.listErr
	}

	var matched []models.Entry
	for i := len(f.all) - 1; i >= 0; i-- { // newest first, matches ORDER BY id DESC
		if f.all[i].AccountID == accountID {
			matched = append(matched, f.all[i])
		}
	}

	if offset >= len(matched) {
		return nil, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end], nil
}

func (f *fakeEntryRepo) CountByAccountID(ctx context.Context, accountID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.countErr != nil {
		return 0, f.countErr
	}

	var count int64
	for _, e := range f.all {
		if e.AccountID == accountID {
			count++
		}
	}
	return count, nil
}

type fakeIdempotencyRepo struct {
	mu     sync.Mutex
	byKey  map[string]*models.IdempotencyKey
	nextID int64

	findOrCreateErr error
	completeErr     error
}

func newFakeIdempotencyRepo() *fakeIdempotencyRepo {
	return &fakeIdempotencyRepo{byKey: map[string]*models.IdempotencyKey{}}
}

func idemKey(userID int64, key string) string { return fmt.Sprintf("%d:%s", userID, key) }

func (f *fakeIdempotencyRepo) FindOrCreate(ctx context.Context, userID int64, key, requestHash string) (*models.IdempotencyKey, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.findOrCreateErr != nil {
		return nil, false, f.findOrCreateErr
	}

	k := idemKey(userID, key)
	if existing, ok := f.byKey[k]; ok {
		cp := *existing
		return &cp, false, nil
	}

	f.nextID++
	rec := &models.IdempotencyKey{
		ID:             f.nextID,
		UserID:         userID,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		CreatedAt:      time.Now(),
	}
	f.byKey[k] = rec

	cp := *rec
	return &cp, true, nil
}

func (f *fakeIdempotencyRepo) Complete(ctx context.Context, id int64, status int, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.completeErr != nil {
		return f.completeErr
	}

	for _, rec := range f.byKey {
		if rec.ID == id {
			rec.ResponseStatus = &status
			rec.ResponseBody = body
			return nil
		}
	}
	return errNotFound
}

type fakeTokenManager struct {
	generateErr error
}

func (f fakeTokenManager) Generate(userID int64, email string) (string, time.Time, error) {
	if f.generateErr != nil {
		return "", time.Time{}, f.generateErr
	}
	return fmt.Sprintf("token-for-%d", userID), time.Now().Add(time.Hour), nil
}
