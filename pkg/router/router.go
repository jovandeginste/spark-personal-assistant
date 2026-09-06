package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
)

// Metadata represents message headers similar to email.
type Metadata struct {
	From    string         `json:"from"`
	To      []string       `json:"to"`
	Subject string         `json:"subject"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// Message represents a message passing through the router.
type Message struct {
	Metadata Metadata `json:"metadata"`
	Content  string   `json:"content"`
}

// Address represents a registered system instance address in the router.
type Address struct {
	ID           string                                      `json:"id"`
	InstanceName string                                      `json:"instance_name"`
	System       string                                      `json:"system"`
	Description  string                                      `json:"description"`
	SendFunc     func(ctx context.Context, msg Message) error `json:"-"`
}

// Router is the central message router connecting all in- and output systems.
type Router struct {
	mu        sync.RWMutex
	addresses map[string]Address
	inbound   chan Message
	aiClient  ai.Client
	app       *app.App
}

// NewRouter creates a new central message router.
func NewRouter(aiClient ai.Client, a *app.App) *Router {
	return &Router{
		addresses: make(map[string]Address),
		inbound:   make(chan Message, 1000),
		aiClient:  aiClient,
		app:       a,
	}
}

// RegisterAddress registers an address in the router, ensuring instancename@servicename format.
func (r *Router) RegisterAddress(addr Address) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if addr.ID == "" {
		return errors.New("address ID cannot be empty")
	}
	if addr.System == "" {
		return errors.New("address system cannot be empty")
	}

	expectedID := fmt.Sprintf("%s@%s", addr.InstanceName, addr.System)
	if addr.InstanceName == "" {
		return errors.New("address instance name cannot be empty")
	}
	if addr.ID != expectedID {
		return fmt.Errorf("invalid address ID format: got %q, expected %q (instancename@servicename)", addr.ID, expectedID)
	}

	r.addresses[addr.ID] = addr
	if r.app != nil && r.app.Logger() != nil {
		r.app.Logger().Info("Registered address", "id", addr.ID, "system", addr.System, "instance", addr.InstanceName)
	}
	return nil
}

// GetAddress returns a registered address by ID.
func (r *Router) GetAddress(id string) (Address, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	addr, ok := r.addresses[id]
	return addr, ok
}

// ListAddresses returns all registered addresses.
func (r *Router) ListAddresses() []Address {
	r.mu.RLock()
	defer r.mu.RUnlock()
	addrs := make([]Address, 0, len(r.addresses))
	for _, addr := range r.addresses {
		addrs = append(addrs, addr)
	}
	return addrs
}

// MCPTools returns the router tools to be exposed to the LLM via MCP / tool executor.
func (r *Router) MCPTools() []ai.Tool {
	return []ai.Tool{
		{
			Name:        "router_list_destinations",
			Description: "List all available message destinations/addresses registered in the router (in format instancename@servicename).",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "router_send_message",
			Description: "Send a message via the router to one or more destination addresses (instancename@servicename).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of destination address IDs (instancename@servicename)",
					},
					"subject": map[string]any{
						"type":        "string",
						"description": "Subject of the message",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content/body of the message",
					},
				},
				"required": []any{"to", "content"},
			},
		},
	}
}

// ExecuteTool executes router MCP tools. Returns (result string, handled bool, err error).
func (r *Router) ExecuteTool(ctx context.Context, name string, args map[string]any, originatingMessage Message) (string, bool, error) {
	switch name {
	case "router_list_destinations":
		return r.executeListDestinations(), true, nil
	case "router_send_message":
		res, err := r.executeSendMessage(ctx, originatingMessage, args)
		return res, true, err
	}
	return "", false, nil
}

func (r *Router) executeListDestinations() string {
	addrs := r.ListAddresses()
	var sb strings.Builder
	sb.WriteString("Available destinations:\n")
	for _, a := range addrs {
		sb.WriteString(fmt.Sprintf("- ID: %s, System: %s, Description: %s\n", a.ID, a.System, a.Description))
	}
	return sb.String()
}

func (r *Router) executeSendMessage(ctx context.Context, originatingMessage Message, args map[string]any) (string, error) {
	toRaw, _ := args["to"].([]any)
	var to []string
	for _, t := range toRaw {
		if str, ok := t.(string); ok {
			to = append(to, str)
		}
	}
	subject, _ := args["subject"].(string)
	content, _ := args["content"].(string)

	outMsg := Message{
		Metadata: Metadata{
			From:    "ai",
			To:      to,
			Subject: subject,
			Extra:   originatingMessage.Metadata.Extra,
		},
		Content: content,
	}

	err := r.SubmitMessage(ctx, "ai", outMsg)
	if err != nil {
		return "", err
	}
	return "Message successfully sent to " + strings.Join(to, ", "), nil
}

// SubmitMessage receives an incoming message from any system instance, sets the "from" address, and routes it.
func (r *Router) SubmitMessage(ctx context.Context, sourceID string, msg Message) error {
	sourceAddr, err := r.validateAndSetSource(sourceID, &msg)
	if err != nil {
		return err
	}

	r.logMessage(&msg)
	r.handleAIRequirement(ctx, &msg)
	r.dispatchToTargets(ctx, &msg)

	_ = sourceAddr
	return nil
}

func (r *Router) validateAndSetSource(sourceID string, msg *Message) (Address, error) {
	r.mu.RLock()
	sourceAddr, ok := r.addresses[sourceID]
	r.mu.RUnlock()

	if !ok {
		return Address{}, fmt.Errorf("unknown source address: %s", sourceID)
	}

	msg.Metadata.From = sourceAddr.ID
	return sourceAddr, nil
}

func (r *Router) logMessage(msg *Message) {
	if r.app != nil && r.app.Logger() != nil {
		r.app.Logger().Info("Message received by router", "from", msg.Metadata.From, "to", msg.Metadata.To, "subject", msg.Metadata.Subject)
	}
}

func (r *Router) handleAIRequirement(ctx context.Context, msg *Message) {
	needsAI := false
	for _, target := range msg.Metadata.To {
		if isAITarget(target) {
			needsAI = true
			break
		}
	}
	if len(msg.Metadata.To) == 0 && !isAIAsync(msg.Metadata.From) {
		needsAI = true
		msg.Metadata.To = []string{"ai"}
	}

	if needsAI && r.aiClient != nil {
		go r.routeToAI(ctx, *msg)
	}
}

func isAITarget(target string) bool {
	return target == "ai" || target == "LLM" || target == "llm" || target == "AI"
}

func isAIAsync(from string) bool {
	return from == "ai" || from == "LLM" || from == "llm" || from == "AI"
}

func (r *Router) dispatchToTargets(ctx context.Context, msg *Message) {
	for _, targetID := range msg.Metadata.To {
		if isAITarget(targetID) {
			continue
		}

		r.mu.RLock()
		targetAddr, ok := r.addresses[targetID]
		r.mu.RUnlock()

		if !ok {
			if r.app != nil && r.app.Logger() != nil {
				r.app.Logger().Warn("Unknown target address", "target", targetID)
			}
			continue
		}

		if targetAddr.SendFunc != nil {
			if err := targetAddr.SendFunc(ctx, *msg); err != nil && r.app != nil && r.app.Logger() != nil {
				r.app.Logger().Error("Failed to send message to address", "target", targetAddr.ID, "error", err)
			}
		}
	}
}

func (r *Router) routeToAI(ctx context.Context, msg Message) {
	if r.app == nil || r.aiClient == nil {
		return
	}

	roomID := ""
	if msg.Metadata.Extra != nil {
		if rID, ok := msg.Metadata.Extra["room_id"].(string); ok {
			roomID = rID
		}
	}

	tools, err := r.app.GetMCPTools(ctx, roomID)
	if err != nil {
		r.app.Logger().Error("Failed to get MCP tools", "error", err)
	}

	// Append router MCP tools
	tools = append(tools, r.MCPTools()...)

	aiData, err := r.app.BuildData()
	if err != nil {
		r.app.Logger().Error("Failed to build AI data", "error", err)
	}
	if aiData != nil {
		aiData.EmployerQuestion = []string{fmt.Sprintf("From: %s\nSubject: %s\n\n%s", msg.Metadata.From, msg.Metadata.Subject, msg.Content)}
	}

	response, err := r.aiClient.GenerateWithTools(ctx, aiData, tools, func(ctx context.Context, name string, args map[string]any) (string, error) {
		if res, handled, err := r.ExecuteTool(ctx, name, args, msg); handled {
			return res, err
		}
		return r.app.ExecuteMCPTool(ctx, name, args, roomID)
	}, nil)

	if err != nil {
		r.app.Logger().Error("Failed to get AI response", "error", err)
		r.sendReply(ctx, msg.Metadata.From, "Sorry, I encountered an error processing your request.", msg.Metadata.Subject, msg.Metadata.Extra)
		return
	}

	if response != "" {
		r.sendReply(ctx, msg.Metadata.From, response, msg.Metadata.Subject, msg.Metadata.Extra)
	}
}

func (r *Router) sendReply(ctx context.Context, targetID, content, subject string, extra map[string]any) {
	r.mu.RLock()
	targetAddr, ok := r.addresses[targetID]
	r.mu.RUnlock()

	if !ok {
		return
	}

	msg := Message{
		Metadata: Metadata{
			From:    "ai",
			To:      []string{targetID},
			Subject: subject,
			Extra:   extra,
		},
		Content: content,
	}

	if targetAddr.SendFunc != nil {
		_ = targetAddr.SendFunc(ctx, msg)
	}
}
