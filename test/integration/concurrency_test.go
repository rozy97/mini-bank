//go:build integration

// This file proves what the fake-backed unit tests structurally cannot: that
// the real SELECT ... FOR UPDATE locking (in ascending account-ID order)
// actually prevents lost updates and deadlocks when many transfers race
// against the same pair of accounts on a real Postgres.
package integration_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransfer_ConcurrentBackAndForthConservesTotalBalance(t *testing.T) {
	_, accountAID, tokenA := registerUser(t, "race-a")
	_, accountBID, tokenB := registerUser(t, "race-b")

	const (
		workers     = 20
		perWorker   = 10
		perTransfer = int64(10_000)
	)
	totalBefore := accountBalance(t, accountAID) + accountBalance(t, accountBID)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			fromToken, toAccountID := tokenA, accountBID
			if i%2 == 1 {
				fromToken, toAccountID = tokenB, accountAID
			}

			for j := 0; j < perWorker; j++ {
				resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
					"to_account_id": toAccountID,
					"amount":        perTransfer,
				}, map[string]string{"Idempotency-Key": uuid.NewString()})

				// Running low on funds from the back-and-forth traffic is an
				// expected outcome under this load, not a bug. Anything else
				// (a deadlock error surfacing as 500, a bad lock order
				// corrupting state, ...) is.
				if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusUnprocessableEntity {
					t.Errorf("unexpected status %d for transfer %d.%d: %+v", resp.StatusCode, i, j, body.Error)
				}
			}
		}(i)
	}
	wg.Wait()

	balanceA := accountBalance(t, accountAID)
	balanceB := accountBalance(t, accountBID)

	assert.Equal(t, totalBefore, balanceA+balanceB, "concurrent transfers must conserve the total balance across both accounts")
	assert.GreaterOrEqual(t, balanceA, int64(0), "balance must never go negative")
	assert.GreaterOrEqual(t, balanceB, int64(0), "balance must never go negative")
}

// TestTransfer_ConcurrentIdenticalRetries fires the same idempotency key
// concurrently, the way a client's retried request and its original might
// race in flight at the same time. Because idempotency-key reservation and
// completion happen in the same database transaction as the transfer
// itself, every concurrent attempt must either do the work or block until
// the one that did commits and then read its (already complete) cached
// result — so every response must report the same transfer, and the
// transfer must be applied exactly once.
func TestTransfer_ConcurrentIdenticalRetries(t *testing.T) {
	_, fromAccountID, fromToken := registerUser(t, "race-idem-from")
	_, toAccountID, _ := registerUser(t, "race-idem-to")
	key := uuid.NewString()

	const attempts = 10
	transferIDs := make([]int64, attempts)
	statuses := make([]int, attempts)

	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
				"to_account_id": toAccountID,
				"amount":        1_000_000,
			}, map[string]string{"Idempotency-Key": key})
			statuses[i] = resp.StatusCode
			if resp.StatusCode == http.StatusCreated {
				var data struct {
					TransferID int64 `json:"transfer_id"`
				}
				require.NoError(t, json.Unmarshal(body.Data, &data))
				transferIDs[i] = data.TransferID
			}
		}(i)
	}
	wg.Wait()

	for i, status := range statuses {
		assert.Equal(t, http.StatusCreated, status, "attempt %d", i)
	}
	for i, id := range transferIDs {
		assert.Equal(t, transferIDs[0], id, "attempt %d must report the same transfer as attempt 0", i)
	}

	assert.Equal(t, int64(9_000_000), accountBalance(t, fromAccountID), "the transfer must have been applied exactly once")
}
