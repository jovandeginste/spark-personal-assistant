package drillster

import (
	"log/slog"
	"os"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestDrillster_Register(t *testing.T) {
	config := Config{
		TokenFile: "test-token.json",
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

func TestDrillster_readToken(t *testing.T) {
	// Create a temporary token file
	file, err := os.CreateTemp("", "token-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	tokenData := `{"access_token": "test-token"}`
	if _, err := file.WriteString(tokenData); err != nil {
		t.Fatal(err)
	}
	file.Close()

	config := Config{
		TokenFile: file.Name(),
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	d := New(config, logger)

	token, err := d.readToken()
	assert.NoError(t, err)
	assert.Equal(t, "test-token", token)
}

func TestDrillster_readToken_MissingFile(t *testing.T) {
	config := Config{
		TokenFile: "non-existent-file.json",
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	d := New(config, logger)

	_, err := d.readToken()
	assert.Error(t, err)
}
