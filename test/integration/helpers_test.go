//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Meta    json.RawMessage `json:"meta"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// doRequest sends a request to the running test server. token, body, and
// headers are all optional (pass "", nil, nil to omit).
func doRequest(t *testing.T, method, path, token string, body any, headers map[string]string) (*http.Response, apiResponse) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, testServer.URL+path, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed apiResponse
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &parsed))
	}

	return resp, parsed
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
}

const testPassword = "password123"

// registerUser registers a fresh user (unique email per call) and returns
// its user ID, account ID, and a bearer token from a subsequent login.
func registerUser(t *testing.T, prefix string) (userID, accountID int64, token string) {
	t.Helper()
	email := uniqueEmail(prefix)

	resp, body := doRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name":     "Integration Test",
		"email":    email,
		"password": testPassword,
	}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "register failed: %+v", body.Error)

	var data struct {
		UserID    int64 `json:"user_id"`
		AccountID int64 `json:"account_id"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &data))

	_, loginBody := doRequest(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email":    email,
		"password": testPassword,
	}, nil)

	var loginData struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(loginBody.Data, &loginData))

	return data.UserID, data.AccountID, loginData.AccessToken
}

func accountBalance(t *testing.T, accountID int64) int64 {
	t.Helper()
	var balance int64
	require.NoError(t, testDB.Get(&balance, "SELECT balance FROM accounts WHERE id = $1", accountID))
	return balance
}
