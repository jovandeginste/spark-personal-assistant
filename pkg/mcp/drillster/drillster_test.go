package drillster

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeRequestFailedRelogin(t *testing.T) {
	originalIdentify := drillsterIdentifyURL
	originalAuthenticate := drillsterAuthenticateURL
	originalToken := drillsterSessionURL

	defer func() {
		drillsterIdentifyURL = originalIdentify
		drillsterAuthenticateURL = originalAuthenticate
		drillsterSessionURL = originalToken
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	mux.HandleFunc("/daas/identify", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<input name="_csrf" value="csrf-token">`)
	})
	mux.HandleFunc("/daas/authenticate/password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/tmb/token", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no token", http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	drillsterIdentifyURL = server.URL + "/daas/identify"
	drillsterAuthenticateURL = server.URL + "/daas/authenticate/password"
	drillsterSessionURL = server.URL + "/tmb/token"

	d := New(Config{Username: "user", Password: "pass"}, slog.Default())
	d.token = "expired"

	result, _, err := d.makeRequest(server.URL + "/api")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)

	text, ok := result.Content[0].(*sdk.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "Error re-logging in")
}

func TestDrillster_Register(t *testing.T) {
	config := Config{
		Username: "test-user",
		Password: "test-password",
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	d := New(config, logger)

	server := sdk.NewServer(&sdk.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, &sdk.ServerOptions{
		Logger: logger,
	})

	err := d.Register(server)
	assert.NoError(t, err)
}
