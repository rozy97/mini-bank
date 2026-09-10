//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransfer_Success(t *testing.T) {
	_, fromAccountID, fromToken := registerUser(t, "xfer-from")
	_, toAccountID, _ := registerUser(t, "xfer-to")

	resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
		"to_account_id": toAccountID,
		"amount":        1_000_000,
		"description":   "integration test",
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	require.Equal(t, http.StatusCreated, resp.StatusCode, "transfer failed: %+v", body.Error)

	var data struct {
		TransferID int64 `json:"transfer_id"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &data))
	assert.NotZero(t, data.TransferID)

	assert.Equal(t, int64(9_000_000), accountBalance(t, fromAccountID))
	assert.Equal(t, int64(11_000_000), accountBalance(t, toAccountID))
}

func TestTransfer_IdempotentRetryIsNotAppliedTwice(t *testing.T) {
	_, fromAccountID, fromToken := registerUser(t, "idem-from")
	_, toAccountID, _ := registerUser(t, "idem-to")
	key := uuid.NewString()

	body := map[string]any{"to_account_id": toAccountID, "amount": 500_000}

	resp1, respBody1 := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, body, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	resp2, respBody2 := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, body, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	var t1, t2 struct {
		TransferID int64 `json:"transfer_id"`
	}
	require.NoError(t, json.Unmarshal(respBody1.Data, &t1))
	require.NoError(t, json.Unmarshal(respBody2.Data, &t2))
	assert.Equal(t, t1.TransferID, t2.TransferID, "retrying the same idempotency key must return the original transfer")

	assert.Equal(t, int64(9_500_000), accountBalance(t, fromAccountID), "balance must reflect exactly one transfer, not two")
}

func TestTransfer_IdempotencyKeyReusedWithDifferentPayload(t *testing.T) {
	_, _, fromToken := registerUser(t, "idemconflict-from")
	_, toAccountID, _ := registerUser(t, "idemconflict-to")
	key := uuid.NewString()

	resp, _ := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
		"to_account_id": toAccountID, "amount": 100_000,
	}, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
		"to_account_id": toAccountID, "amount": 200_000,
	}, map[string]string{"Idempotency-Key": key})
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "IDEMPOTENCY_KEY_CONFLICT", body.Error.Code)
}

func TestTransfer_MissingIdempotencyKey(t *testing.T) {
	_, _, fromToken := registerUser(t, "nokey-from")
	_, toAccountID, _ := registerUser(t, "nokey-to")

	resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
		"to_account_id": toAccountID, "amount": 1_000,
	}, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
}

func TestTransfer_InsufficientBalance(t *testing.T) {
	_, _, fromToken := registerUser(t, "poor-from")
	_, toAccountID, _ := registerUser(t, "poor-to")

	resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
		"to_account_id": toAccountID, "amount": 99_000_000,
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "INSUFFICIENT_BALANCE", body.Error.Code)
}

func TestTransfer_SameAccount(t *testing.T) {
	_, accountID, token := registerUser(t, "self")

	resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", token, map[string]any{
		"to_account_id": accountID, "amount": 1_000,
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
}

func TestTransfer_ToNonexistentAccount(t *testing.T) {
	_, _, token := registerUser(t, "ghost-target")

	resp, body := doRequest(t, http.MethodPost, "/api/v1/transfers", token, map[string]any{
		"to_account_id": 999_999_999, "amount": 1_000,
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "ACCOUNT_NOT_FOUND", body.Error.Code)
}

func TestTransfer_AppearsInBothAccountsHistory(t *testing.T) {
	_, _, fromToken := registerUser(t, "hist-from")
	_, toAccountID, toToken := registerUser(t, "hist-to")

	resp, _ := doRequest(t, http.MethodPost, "/api/v1/transfers", fromToken, map[string]any{
		"to_account_id": toAccountID, "amount": 250_000, "description": "history check",
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, body := doRequest(t, http.MethodGet, "/api/v1/accounts/me/history?page=1&page_size=10", fromToken, nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var fromEntries []struct {
		Amount int64 `json:"amount"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &fromEntries))
	require.NotEmpty(t, fromEntries)
	assert.Equal(t, int64(-250_000), fromEntries[0].Amount)

	_, body = doRequest(t, http.MethodGet, "/api/v1/accounts/me/history?page=1&page_size=10", toToken, nil, nil)
	var toEntries []struct {
		Amount int64 `json:"amount"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &toEntries))
	require.NotEmpty(t, toEntries)
	assert.Equal(t, int64(250_000), toEntries[0].Amount)
}
