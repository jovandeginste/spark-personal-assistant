package timers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/netresearch/go-cron"
)

type Config struct {
	File     string `mapstructure:"file"`
	Callback string `mapstructure:"callback"`
}

type Timer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Message  string `json:"message"`
	Cron     string `json:"cron"`
	NextTime string `json:"next_time,omitempty"`
}

type Module struct {
	sparkmcp.BaseModule
	mu sync.RWMutex
}

func New(config Config, logger *slog.Logger) *Module {
	return &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "timers")),
	}
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.File == "" {
		return errors.New("timers file is not configured")
	}

	// Ensure the directory exists
	dir := filepath.Dir(config.File)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for timers file: %w", err)
	}

	// Create file if it doesn't exist
	if _, err := os.Stat(config.File); os.IsNotExist(err) {
		err = m.writeTimers(map[string]Timer{})
		if err != nil {
			return fmt.Errorf("failed to create initial timers file: %w", err)
		}
	}

	return nil
}

func (m *Module) readTimers() (map[string]Timer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := m.Config().(Config)
	data, err := os.ReadFile(config.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read timers file: %w", err)
	}

	var timers map[string]Timer
	if err := json.Unmarshal(data, &timers); err != nil {
		return nil, fmt.Errorf("failed to parse timers file: %w", err)
	}

	return timers, nil
}

func (m *Module) Initialize() error {
	m.StartWorker()
	return nil
}

func (m *Module) StartWorker() {
	config := m.Config().(Config)
	logger := m.Logger()

	if config.Callback == "" {
		logger.Info("No callback configured for timers module, disabling worker")
		return
	}

	go func() {
		logger.Info("Starting timer worker", "callback", config.Callback)
		for {
			m.processTimers(config, logger)

			// Sleep until the next minute starts
			now := time.Now()
			nextMinute := now.Truncate(time.Minute).Add(time.Minute)
			time.Sleep(time.Until(nextMinute))
		}
	}()
}

func (m *Module) processTimers(config Config, logger *slog.Logger) {
	now := time.Now()
	timers, err := m.readTimers()
	if err != nil {
		logger.Error("Worker failed to read timers", "error", err)
		return
	}

	parser := cron.MustNewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	var toDelete []string
	var toTrigger []Timer

	for id, timer := range timers {
		if m.shouldTriggerTimer(timer, parser, now) {
			toTrigger = append(toTrigger, timer)
			if _, err := parser.Parse(timer.Cron); err != nil {
				// It's a one-time timer (parsing as cron failed), so delete it after triggering
				toDelete = append(toDelete, id)
			}
		}
	}

	for _, timer := range toTrigger {
		m.triggerCallback(config.Callback, timer, logger)
	}

	if len(toDelete) > 0 {
		m.deleteTimers(config.File, toDelete)
	}
}

func (m *Module) shouldTriggerTimer(timer Timer, parser cron.Parser, now time.Time) bool {
	schedule, err := parser.Parse(timer.Cron)
	if err == nil {
		nowMinute := now.Truncate(time.Minute)
		nextRun := schedule.Next(nowMinute.Add(-1 * time.Minute))
		return nextRun.Equal(nowMinute)
	}

	t, err := time.Parse(time.RFC3339, timer.Cron)
	if err == nil {
		return !t.After(now)
	}

	return false
}

func (m *Module) triggerCallback(callbackTemplate string, timer Timer, logger *slog.Logger) {
	nameSlug := slug.Make(timer.Name)
	// Add prefix to the slug
	nameSlug = fmt.Sprintf("[timers]-%s", nameSlug)
	callbackUrl := strings.ReplaceAll(callbackTemplate, "_NAME_", nameSlug)
	
	// URL-encode the message
	encodedMessage := url.QueryEscape(timer.Message)
	callbackUrl = strings.ReplaceAll(callbackUrl, "_MESSAGE_", encodedMessage)
	
	logger.Info("Triggering timer", "id", timer.ID, "name", timer.Name, "slug", nameSlug, "url", callbackUrl)
	go func(u string) {
		// #nosec G107
		resp, err := http.Get(u)
		if err != nil {
			logger.Error("Failed to call callback", "url", u, "error", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			logger.Error("Callback returned error status", "url", u, "status", resp.Status)
		}
	}(callbackUrl)
}

func (m *Module) deleteTimers(file string, toDelete []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(file)
	if err != nil {
		return
	}

	var currentTimers map[string]Timer
	if err := json.Unmarshal(data, &currentTimers); err != nil {
		return
	}

	for _, id := range toDelete {
		delete(currentTimers, id)
	}

	outData, err := json.MarshalIndent(currentTimers, "", "  ")
	if err != nil {
		return
	}

	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, outData, 0o600); err == nil {
		_ = os.Rename(tmpFile, file)
	}
}

func (m *Module) writeTimers(timers map[string]Timer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config := m.Config().(Config)
	data, err := json.MarshalIndent(timers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal timers: %w", err)
	}

	tmpFile := config.File + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary timers file: %w", err)
	}

	if err := os.Rename(tmpFile, config.File); err != nil {
		os.Remove(tmpFile) // clean up temp file on error
		return fmt.Errorf("failed to move timers file to final location: %w", err)
	}

	return nil
}

type createTimerParams struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Cron    string `json:"cron"`
}

type deleteTimerParams struct {
	ID string `json:"id"`
}

func (m *Module) Register(server *mcp.Server) error {
	m.Logger().Info("Registering Timers MCP tools")

	mcp.AddTool(server, &mcp.Tool{
		Name:        "timers_list",
		Description: "List all configured timers",
	}, m.handleListTimers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "timers_create",
		Description: "Create a new timer",
	}, m.handleCreateTimer)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "timers_delete",
		Description: "Delete a timer",
	}, m.handleDeleteTimer)

	return nil
}

func (m *Module) handleListTimers(ctx context.Context, request *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
	timers, err := m.readTimers()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading timers: %w", err)
	}

	if len(timers) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No timers found."},
			},
		}, nil, nil
	}

	now := time.Now()

	parser := cron.MustNewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	for id, timer := range timers {
		schedule, err := parser.Parse(timer.Cron)
		if err == nil {
			timer.NextTime = schedule.Next(now).Format(time.RFC3339)
			timers[id] = timer
		} else {
			// Might be a one-time date? Let's check if we can parse it as RFC3339
			t, err := time.Parse(time.RFC3339, timer.Cron)
			if err == nil {
				if t.After(now) {
					timer.NextTime = t.Format(time.RFC3339)
				} else {
					timer.NextTime = "Past"
				}
				timers[id] = timer
			} else {
				timer.NextTime = "Invalid schedule"
				timers[id] = timer
			}
		}
	}

	data, err := json.MarshalIndent(timers, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("error marshaling timers: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		},
	}, nil, nil
}

func (m *Module) handleCreateTimer(ctx context.Context, request *mcp.CallToolRequest, params createTimerParams) (*mcp.CallToolResult, any, error) {
	if params.ID == "" {
		return nil, nil, errors.New("missing or invalid 'id'")
	}
	if params.Name == "" {
		return nil, nil, errors.New("missing or invalid 'name'")
	}
	if params.Message == "" {
		return nil, nil, errors.New("missing or invalid 'message'")
	}
	if params.Cron == "" {
		return nil, nil, errors.New("missing or invalid 'cron'")
	}

	timers, err := m.readTimers()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading timers: %w", err)
	}

	if _, exists := timers[params.ID]; exists {
		return nil, nil, fmt.Errorf("timer with ID '%s' already exists", params.ID)
	}

	timer := Timer{
		ID:      params.ID,
		Name:    params.Name,
		Message: params.Message,
		Cron:    params.Cron,
	}
	timers[params.ID] = timer

	if err := m.writeTimers(timers); err != nil {
		return nil, nil, fmt.Errorf("error writing timers: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Timer '%s' created successfully", params.ID)},
		},
	}, nil, nil
}

func (m *Module) handleDeleteTimer(ctx context.Context, request *mcp.CallToolRequest, params deleteTimerParams) (*mcp.CallToolResult, any, error) {
	if params.ID == "" {
		return nil, nil, errors.New("missing or invalid 'id'")
	}

	timers, err := m.readTimers()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading timers: %w", err)
	}

	if _, exists := timers[params.ID]; !exists {
		return nil, nil, fmt.Errorf("timer with ID '%s' not found", params.ID)
	}

	delete(timers, params.ID)

	if err := m.writeTimers(timers); err != nil {
		return nil, nil, fmt.Errorf("error writing timers: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Timer '%s' deleted successfully", params.ID)},
		},
	}, nil, nil
}
