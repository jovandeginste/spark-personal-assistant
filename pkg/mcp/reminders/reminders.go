package reminders

import (
	"bytes"
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

type Reminder struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Message  string `json:"message"`
	Cron     string `json:"cron"`
	OneTime  bool   `json:"one_time"`
	NextTime string `json:"next_time,omitempty"`
}

type Module struct {
	sparkmcp.BaseModule
	mu sync.RWMutex
}

func New(config Config, logger *slog.Logger) *Module {
	return &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "reminders")),
	}
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.File == "" {
		return errors.New("reminders file is not configured")
	}

	// Ensure the directory exists
	dir := filepath.Dir(config.File)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for reminders file: %w", err)
	}

	// Create file if it doesn't exist
	if _, err := os.Stat(config.File); os.IsNotExist(err) {
		err = m.writeReminders(map[string]Reminder{})
		if err != nil {
			return fmt.Errorf("failed to create initial reminders file: %w", err)
		}
	}

	return nil
}

func (m *Module) readReminders() (map[string]Reminder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := m.Config().(Config)
	data, err := os.ReadFile(config.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read reminders file: %w", err)
	}

	var reminders map[string]Reminder
	if err := json.Unmarshal(data, &reminders); err != nil {
		return nil, fmt.Errorf("failed to parse reminders file: %w", err)
	}

	return reminders, nil
}

func (m *Module) Initialize() error {
	m.StartWorker()
	return nil
}

func (m *Module) StartWorker() {
	config := m.Config().(Config)
	logger := m.Logger()

	if config.Callback == "" {
		logger.Info("No callback configured for reminders module, disabling worker")
		return
	}

	go func() {
		logger.Info("Starting reminder worker", "callback", config.Callback)
		for {
			m.processReminders(config, logger)

			// Sleep until the next minute starts
			now := time.Now()
			nextMinute := now.Truncate(time.Minute).Add(time.Minute)
			time.Sleep(time.Until(nextMinute))
		}
	}()
}

func (m *Module) processReminders(config Config, logger *slog.Logger) {
	now := time.Now()
	reminders, err := m.readReminders()
	if err != nil {
		logger.Error("Worker failed to read reminders", "error", err)
		return
	}

	parser := cron.MustNewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	var toDelete []string
	var toTrigger []Reminder

	for id, reminder := range reminders {
		if m.shouldTriggerReminder(reminder, parser, now) {
			toTrigger = append(toTrigger, reminder)
			_, err := parser.Parse(reminder.Cron)
			if reminder.OneTime || err != nil {
				// It's a one-time reminder (explicitly requested or parsing as cron failed), so delete it after triggering
				toDelete = append(toDelete, id)
			}
		}
	}

	for _, reminder := range toTrigger {
		m.triggerCallback(config.Callback, reminder, logger)
	}

	if len(toDelete) > 0 {
		m.deleteReminders(config.File, toDelete)
	}
}

func (m *Module) shouldTriggerReminder(reminder Reminder, parser cron.Parser, now time.Time) bool {
	schedule, err := parser.Parse(reminder.Cron)
	if err == nil {
		nowMinute := now.Truncate(time.Minute)
		nextRun := schedule.Next(nowMinute.Add(-1 * time.Minute))
		return nextRun.Equal(nowMinute)
	}

	t, err := time.Parse(time.RFC3339, reminder.Cron)
	if err == nil {
		return !t.After(now)
	}

	return false
}

func (m *Module) triggerCallback(callbackTemplate string, reminder Reminder, logger *slog.Logger) {
	nameSlug := slug.Make(reminder.Name)
	// Add prefix to the slug
	nameSlug = "[reminders]-" + nameSlug
	callbackUrl := strings.ReplaceAll(callbackTemplate, "_NAME_", nameSlug)

	// Keep the original URL replacement for backward compatibility, but we will mostly rely on POST body now
	encodedMessage := url.QueryEscape(reminder.Message)
	callbackUrl = strings.ReplaceAll(callbackUrl, "_MESSAGE_", encodedMessage)

	logger.Info("Triggering reminder", "id", reminder.ID, "name", reminder.Name, "slug", nameSlug, "url", callbackUrl)

	go func(u string, r Reminder) {
		payload := map[string]string{
			"title":   r.Name,
			"message": r.Message,
			"source":  "reminders",
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			logger.Error("Failed to marshal reminder payload", "error", err)
			return
		}

		// #nosec G107
		resp, err := http.Post(u, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Error("Failed to call callback", "url", u, "error", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			logger.Error("Callback returned error status", "url", u, "status", resp.Status)
		}
	}(callbackUrl, reminder)
}

func (m *Module) deleteReminders(file string, toDelete []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(file)
	if err != nil {
		return
	}

	var currentReminders map[string]Reminder
	if err := json.Unmarshal(data, &currentReminders); err != nil {
		return
	}

	for _, id := range toDelete {
		delete(currentReminders, id)
	}

	outData, err := json.MarshalIndent(currentReminders, "", "  ")
	if err != nil {
		return
	}

	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, outData, 0o600); err == nil {
		_ = os.Rename(tmpFile, file)
	}
}

func (m *Module) writeReminders(reminders map[string]Reminder) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config := m.Config().(Config)
	data, err := json.MarshalIndent(reminders, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal reminders: %w", err)
	}

	tmpFile := config.File + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary reminders file: %w", err)
	}

	if err := os.Rename(tmpFile, config.File); err != nil {
		os.Remove(tmpFile) // clean up temp file on error
		return fmt.Errorf("failed to move reminders file to final location: %w", err)
	}

	return nil
}

type createReminderParams struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Cron    string `json:"cron"`
	OneTime bool   `json:"one_time"`
}

type deleteReminderParams struct {
	ID string `json:"id"`
}

func (m *Module) Register(server *mcp.Server) error {
	m.Logger().Info("Registering Reminders MCP tools")

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reminders_list",
		Description: "List all configured reminders",
	}, m.handleListReminders)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reminders_create",
		Description: "Create a new reminder",
	}, m.handleCreateReminder)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reminders_delete",
		Description: "Delete a reminder",
	}, m.handleDeleteReminder)

	return nil
}

func (m *Module) handleListReminders(ctx context.Context, request *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
	reminders, err := m.readReminders()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading reminders: %w", err)
	}

	if len(reminders) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No reminders found."},
			},
		}, nil, nil
	}

	now := time.Now()

	parser := cron.MustNewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	for id, reminder := range reminders {
		schedule, err := parser.Parse(reminder.Cron)
		if err == nil {
			reminder.NextTime = schedule.Next(now).Format(time.RFC3339)
			reminders[id] = reminder
		} else {
			// Might be a one-time date? Let's check if we can parse it as RFC3339
			t, err := time.Parse(time.RFC3339, reminder.Cron)
			if err == nil {
				if t.After(now) {
					reminder.NextTime = t.Format(time.RFC3339)
				} else {
					reminder.NextTime = "Past"
				}
				reminders[id] = reminder
			} else {
				reminder.NextTime = "Invalid schedule"
				reminders[id] = reminder
			}
		}
	}

	data, err := json.MarshalIndent(reminders, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("error marshaling reminders: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		},
	}, nil, nil
}

func (m *Module) handleCreateReminder(ctx context.Context, request *mcp.CallToolRequest, params createReminderParams) (*mcp.CallToolResult, any, error) {
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

	reminders, err := m.readReminders()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading reminders: %w", err)
	}

	if _, exists := reminders[params.ID]; exists {
		return nil, nil, fmt.Errorf("reminder with ID '%s' already exists", params.ID)
	}

	reminder := Reminder{
		ID:      params.ID,
		Name:    params.Name,
		Message: params.Message,
		Cron:    params.Cron,
		OneTime: params.OneTime,
	}
	reminders[params.ID] = reminder

	if err := m.writeReminders(reminders); err != nil {
		return nil, nil, fmt.Errorf("error writing reminders: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Reminder '%s' created successfully", params.ID)},
		},
	}, nil, nil
}

func (m *Module) handleDeleteReminder(ctx context.Context, request *mcp.CallToolRequest, params deleteReminderParams) (*mcp.CallToolResult, any, error) {
	if params.ID == "" {
		return nil, nil, errors.New("missing or invalid 'id'")
	}

	reminders, err := m.readReminders()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading reminders: %w", err)
	}

	if _, exists := reminders[params.ID]; !exists {
		return nil, nil, fmt.Errorf("reminder with ID '%s' not found", params.ID)
	}

	delete(reminders, params.ID)

	if err := m.writeReminders(reminders); err != nil {
		return nil, nil, fmt.Errorf("error writing reminders: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Reminder '%s' deleted successfully", params.ID)},
		},
	}, nil, nil
}
