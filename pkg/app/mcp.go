package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (a *App) InitializeMCP() error {
	// We'll initialize connections on demand or lazily, but let's at least validate config here
	for name, config := range a.Config.MCPServers {
		a.Logger().Info("MCP server configured", "name", name, "type", config.Transport)
		// We can try to connect here to fail fast if config is bad, but for resiliency
		// we should rely on getMCPClient to handle connections and reconnections.
	}

	return nil
}

func (a *App) getMCPClient(ctx context.Context, name string) (*mcp.ClientSession, error) {
	// If we already have a healthy client, return it
	// Note: The SDK doesn't expose a simple "IsConnected()" check that is cheap.
	// We might need to handle this by trying and if it fails, reconnecting.
	// For now, let's just return the existing session if it exists, and let the caller handle errors by retrying.
	// OR: we can check if the session is closed? The struct doesn't export that state easily.

	// Better approach: Since we can't easily check health without making a call,
	// let's wrap the "get" logic to create if missing.
	session, ok := a.mcpClients[name]
	if ok {
		// Basic check - we can't really know if it's dead until we try to use it
		// or if we track close events (which we don't currently).
		// For a robust implementation, we might want to send a ping? Or just return it.
		return session, nil
	}

	return a.connectMCPClient(ctx, name)
}

func (a *App) connectMCPClient(_ context.Context, name string) (*mcp.ClientSession, error) {
	config, ok := a.Config.MCPServers[name]
	if !ok {
		return nil, fmt.Errorf("mcp server config not found: %s", name)
	}

	var transport mcp.Transport

	// Default to stdio if not specified
	if config.Transport == "" {
		if config.URL != "" {
			config.Transport = "sse"
		} else {
			config.Transport = "stdio"
		}
	}

	switch config.Transport {
	case "sse":
		if config.URL == "" {
			return nil, fmt.Errorf("MCP server %s configured with sse transport but no url", name)
		}

		transport = &mcp.SSEClientTransport{
			Endpoint: config.URL,
			HTTPClient: &http.Client{
				Transport: &headerTransport{
					transport: http.DefaultTransport,
				},
			},
		}
	case "stdio":
		if config.Command == "" {
			return nil, fmt.Errorf("MCP server %s configured with stdio transport but no command", name)
		}

		tf := &mcp.CommandTransport{
			Command: exec.Command(config.Command, config.Args...), //nolint:gosec // Trusted config
		}
		tf.Command.Env = os.Environ()
		tf.Command.Env = append(tf.Command.Env, config.Env...)
		tf.Command.Stderr = os.Stderr
		transport = tf
	default:
		return nil, fmt.Errorf("unknown MCP transport for %s: %s", name, config.Transport)
	}

	c := mcp.NewClient(&mcp.Implementation{
		Name:    "spark",
		Version: "1.0.0",
	}, nil)

	a.Logger().Info("Connecting to MCP server", "name", name, "transport", config.Transport)

	// Create a context for the connection.
	// Note: We need a long-lived context for the connection, distinct from the request context.
	// Using context.Background() for now as these should live as long as the app/connection needs.
	connectCtx := context.Background()

	session, err := c.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
	}

	a.mcpClients[name] = session

	// Clean up old cleanup function if exists (though we just overwrote the map entry)
	// We should probably structure this better, but for now:
	cleanup := func() {
		session.Close()
	}
	a.mcpCleanup = append(a.mcpCleanup, cleanup)

	return session, nil
}

// forceReconnect closes the existing session (if any) and removes it, ensuring the next get call creates a new one
func (a *App) forceReconnect(name string) {
	if session, ok := a.mcpClients[name]; ok {
		a.Logger().Info("Forcing reconnection for MCP server", "name", name)
		session.Close()
		delete(a.mcpClients, name)
	}
}

func (a *App) GetMCPTools(ctx context.Context, roomID string) ([]ai.Tool, error) {
	var tools []ai.Tool

	// Iterate over configured servers instead of active clients to ensure we try to connect to all
	for name, config := range a.Config.MCPServers {
		allowed := false
		if len(config.AllowedRooms) == 0 {
			// No rooms allowed
			allowed = false
		} else {
			for _, r := range config.AllowedRooms {
				if r == "*" {
					allowed = true
					break
				}
				if r == roomID {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			continue
		}

		client, err := a.getMCPClient(ctx, name)
		if err != nil {
			a.Logger().Error("Failed to get MCP client", "client", name, "error", err)
			continue
		}

		result, err := client.ListTools(ctx, nil)
		if err != nil {
			a.Logger().Warn("Failed to list tools, retrying connection", "client", name, "error", err)

			// Retry once
			a.forceReconnect(name)

			client, err = a.getMCPClient(ctx, name)
			if err != nil {
				a.Logger().Error("Failed to reconnect to MCP client", "client", name, "error", err)
				continue
			}

			result, err = client.ListTools(ctx, nil)
			if err != nil {
				a.Logger().Error("Failed to list tools after reconnect", "client", name, "error", err)
				continue
			}
		}

		for _, t := range result.Tools {
			tools = append(tools, ai.Tool{
				Name:        name + "__" + t.Name, // Use server name as prefix, not clientName map key which is same
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	return tools, nil
}

func (a *App) ExecuteMCPTool(ctx context.Context, name string, args map[string]any, roomID string) (string, error) {
	a.Logger().Info("executing mcp tool", "tool", name, "args", args)

	parts := strings.SplitN(name, "__", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid tool name format: %s", name)
	}

	clientName, toolName := parts[0], parts[1]

	config, ok := a.Config.MCPServers[clientName]
	if !ok {
		return "", fmt.Errorf("mcp server config not found: %s", clientName)
	}

	allowed := false
	if len(config.AllowedRooms) > 0 {
		for _, r := range config.AllowedRooms {
			if r == "*" {
				allowed = true
				break
			}
			if r == roomID {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		return "", fmt.Errorf("mcp server %s not allowed in room %s", clientName, roomID)
	}

	// Helper to execute the call
	execute := func() (*mcp.CallToolResult, error) {
		client, err := a.getMCPClient(ctx, clientName)
		if err != nil {
			return nil, fmt.Errorf("failed to get mcp client: %w", err)
		}

		return client.CallTool(ctx, &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		})
	}

	result, err := execute()
	if err != nil {
		a.Logger().Warn("mcp tool call failed, retrying connection", "client", clientName, "error", err)
		// Retry once with force reconnect
		a.forceReconnect(clientName)

		result, err = execute()
		if err != nil {
			return "", fmt.Errorf("mcp tool call failed after retry: %w", err)
		}
	}

	if result.IsError {
		jsonBytes, err := json.Marshal(result.Content)
		if err != nil {
			return "", fmt.Errorf("tool execution error (marshal failed): %w", err)
		}
		return "", fmt.Errorf("tool execution error: %s", string(jsonBytes))
	}

	jsonBytes, err := json.Marshal(result.Content)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool result: %w", err)
	}

	return string(jsonBytes), nil
}

func (a *App) UpdateMCPServers(ctx context.Context) map[string]string {
	results := make(map[string]string)
	type result struct {
		name string
		msg  string
	}
	resChan := make(chan result)
	count := 0

	for name, config := range a.Config.MCPServers {
		if config.URL == "" {
			continue
		}
		count++

		go func(name, url string) {
			// Assuming the URL is the base URL, append /update
			updateURL := url
			if updateURL[len(updateURL)-1] == '/' {
				updateURL = updateURL[:len(updateURL)-1]
			}
			// If the URL ends with /sse, strip it
			if len(updateURL) > 4 && updateURL[len(updateURL)-4:] == "/sse" {
				updateURL = updateURL[:len(updateURL)-4]
			}
			updateURL += "/update"

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateURL, nil)
			if err != nil {
				resChan <- result{name, fmt.Sprintf("Failed to create request: %v", err)}
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				resChan <- result{name, fmt.Sprintf("Failed: %v", err)}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				resChan <- result{name, fmt.Sprintf("Failed: Status %d", resp.StatusCode)}
				return
			}

			resChan <- result{name, "Success"}
		}(name, config.URL)
	}

	for i := 0; i < count; i++ {
		res := <-resChan
		results[res.name] = res.msg
	}

	return results
}

type headerTransport struct {
	transport http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Accept", "text/event-stream")

	tr := t.transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	return tr.RoundTrip(r)
}
