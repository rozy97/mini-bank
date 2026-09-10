//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_RegisterPrepopulatesBalance(t *testing.T) {
	_, accountID, _ := registerUser(t, "register")
	assert.Equal(t, int64(10_000_000), accountBalance(t, accountID))
}

func TestAuth_RegisterDuplicateEmail(t *testing.T) {
	email := uniqueEmail("dup")

	resp, _ := doRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "First", "email": email, "password": testPassword,
	}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, body := doRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Second", "email": email, "password": testPassword,
	}, nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "EMAIL_ALREADY_EXISTS", body.Error.Code)
}

func TestAuth_LoginWrongPassword(t *testing.T) {
	email := uniqueEmail("wrongpw")
	resp, _ := doRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Test", "email": email, "password": testPassword,
	}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, body := doRequest(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": "not the right password",
	}, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "INVALID_CREDENTIALS", body.Error.Code)
}

func TestAuth_BalanceRequiresAuthentication(t *testing.T) {
	resp, body := doRequest(t, http.MethodGet, "/api/v1/accounts/me/balance", "", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
}

func TestAuth_BalanceRejectsGarbageToken(t *testing.T) {
	resp, body := doRequest(t, http.MethodGet, "/api/v1/accounts/me/balance", "not-a-real-token", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
}

func TestAuth_GetBalance(t *testing.T) {
	_, _, token := registerUser(t, "balance")

	resp, body := doRequest(t, http.MethodGet, "/api/v1/accounts/me/balance", token, nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var data struct {
		Balance  int64  `json:"balance"`
		Currency string `json:"currency"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &data))
	assert.Equal(t, int64(10_000_000), data.Balance)
	assert.Equal(t, "IDR", data.Currency)
}
