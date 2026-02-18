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
