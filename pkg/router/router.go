package router

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
)

const maxToolArgLengthLog = 40

// Metadata represents message headers similar to email.
type Metadata struct {
	From    string         `json:"from"`
	To      []string       `json:"to"`
	Subject string         `json:"subject"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// Message represents a message passing through the router.
type Message struct {
	Metadata       Metadata `json:"metadata"`
	Content        string   `json:"content"`
	OriginalSource string   `json:"original_source,omitempty"`
}

// Address represents a registered system instance address in the router.
type Address struct {
	ID           string                                       `json:"id"`
	InstanceName string                                       `json:"instance_name"`
	System       string                                       `json:"system"`
	Description  string                                       `json:"description"`
	SendFunc     func(ctx context.Context, msg Message) error `json:"-"`
}

// Router is the central message router connecting all in- and output systems.
type Router struct {
	mu        sync.RWMutex
	addresses map[string]Address
	aiClient  ai.Client
	app       *app.App
}

var (
	listDestinationsSchema = map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	listChannelDestinationsSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"system": map[string]any{
				"type":        "string",
				"description": "The system name (e.g. matrix, mail)",
			},
		},
		"required": []any{"system"},
	}
	sendMessageSchema = map[string]any{
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
			"extra": map[string]any{
				"type":        "object",
				"description": "Extra metadata key-value pairs (e.g. specific recipient email address under 'to', target matrix room ID under 'room_id', etc.)",
			},
		},
		"required": []any{"to", "content"},
	}
)

// NewRouter creates a new central message router.
func NewRouter(aiClient ai.Client, a *app.App) *Router {
	return &Router{
		addresses: make(map[string]Address),
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
			InputSchema: listDestinationsSchema,
		},
		{
			Name:        "router_list_channel_destinations",
			Description: "List further sub-destinations or channel IDs for a specific system instance (e.g. Matrix room IDs for matrix, allowed email contacts for mail).",
			InputSchema: listChannelDestinationsSchema,
		},
		{
			Name:        "router_send_message",
			Description: "Send a message via the router to one or more destination addresses (instancename@servicename).",
			InputSchema: sendMessageSchema,
		},
	}
}

// ExecuteTool executes router MCP tools. Returns (result string, handled bool, err error).
func (r *Router) ExecuteTool(ctx context.Context, name string, args map[string]any, originatingMessage Message) (string, bool, error) {
	switch name {
	case "router_list_destinations":
		return r.executeListDestinations(), true, nil
	case "router_list_channel_destinations":
		system, _ := args["system"].(string)
		return r.executeListChannelDestinations(system), true, nil
	case "router_send_message":
		res, err := r.executeSendMessage(ctx, originatingMessage, args)
		return res, true, err
	}
	return "", false, nil
}

func (r *Router) executeListChannelDestinations(system string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sub-destinations for system %s:\n", system))
	for _, addr := range r.addresses {
		if strings.EqualFold(addr.System, system) {
			sb.WriteString(fmt.Sprintf("- Instance: %s, ID: %s, Description: %s\n", addr.InstanceName, addr.ID, addr.Description))
			if strings.EqualFold(system, "matrix") && r.app != nil {
				if mCfg, ok := r.app.Config.Matrix[addr.InstanceName]; ok && mCfg.RoomID != "" {
					sb.WriteString(fmt.Sprintf("  Default Room ID: %s\n", mCfg.RoomID))
				}
			}
			if (strings.EqualFold(system, "mail") || strings.EqualFold(system, "imap")) && r.app != nil {
				if mailCfg, ok := r.app.Config.Mail[addr.InstanceName]; ok {
					if mailCfg.To != "" {
						sb.WriteString(fmt.Sprintf("  Configured default recipient: %s\n", mailCfg.To))
					}
					if len(mailCfg.Allowlist) > 0 {
						sb.WriteString(fmt.Sprintf("  Allowed recipients: %s\n", strings.Join(mailCfg.Allowlist, ", ")))
					}
				}
			}
		}
	}
	return sb.String()
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

	extra := make(map[string]any)
	for k, v := range originatingMessage.Metadata.Extra {
		extra[k] = v
	}
	if extraArgs, ok := args["extra"].(map[string]any); ok {
		for k, v := range extraArgs {
			extra[k] = v
		}
	}

	outMsg := Message{
		Metadata: Metadata{
			From:    "ai",
			To:      to,
			Subject: subject,
			Extra:   extra,
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
	if err := r.validateAndSetSource(sourceID, &msg); err != nil {
		return err
	}
	if err := r.validateRegisteredTargets(msg.Metadata.To); err != nil {
		return err
	}

	r.logMessage(&msg)
	r.handleAIRequirement(ctx, &msg)
	r.dispatchToTargets(ctx, &msg)

	return nil
}

func (r *Router) validateRegisteredTargets(targets []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, targetID := range targets {
		if isAIIdentifier(targetID) {
			continue
		}

		if _, ok := r.addresses[targetID]; !ok {
			return fmt.Errorf("unknown target address: %s", targetID)
		}
	}

	return nil
}

func (r *Router) validateAndSetSource(sourceID string, msg *Message) error {
	r.mu.RLock()
	sourceAddr, ok := r.addresses[sourceID]
	r.mu.RUnlock()

	if !ok {
		// If sourceID is "ai", we allow it as a virtual system source
		if sourceID == "ai" {
			msg.Metadata.From = "ai"
			return nil
		}
		return fmt.Errorf("unknown source address: %s", sourceID)
	}

	msg.Metadata.From = sourceAddr.ID
	return nil
}

func (r *Router) logMessage(msg *Message) {
	if r.app != nil && r.app.Logger() != nil {
		r.app.Logger().Info("Message received by router",
			"from", msg.Metadata.From,
			"original_source", msg.OriginalSource,
			"to", msg.Metadata.To,
			"subject", msg.Metadata.Subject,
			"metadata", msg.Metadata,
		)
	}
}

func (r *Router) handleAIRequirement(ctx context.Context, msg *Message) {
	needsAI := slices.ContainsFunc(msg.Metadata.To, isAIIdentifier)
	if len(msg.Metadata.To) == 0 && !isAIIdentifier(msg.Metadata.From) {
		needsAI = true
		msg.Metadata.To = []string{"ai"}
	}

	if needsAI && r.aiClient != nil {
		go r.routeToAI(ctx, *msg)
	}
}

func isAIIdentifier(value string) bool {
	return strings.EqualFold(value, "ai") || strings.EqualFold(value, "llm")
}

func (r *Router) dispatchToTargets(ctx context.Context, msg *Message) {
	for _, targetID := range msg.Metadata.To {
		if isAIIdentifier(targetID) {
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
		senderInfo := msg.Metadata.From
		if msg.OriginalSource != "" {
			senderInfo = fmt.Sprintf("%s (%s)", msg.Metadata.From, msg.OriginalSource)
		} else if msg.Metadata.Extra != nil {
			if fromAddr, ok := msg.Metadata.Extra["from_address"].(string); ok && fromAddr != "" {
				senderInfo = fmt.Sprintf("%s (%s)", msg.Metadata.From, fromAddr)
			}
		}
		aiData.EmployerQuestion = []string{fmt.Sprintf("From: %s\nSubject: %s\n\n%s", senderInfo, msg.Metadata.Subject, msg.Content)}
	}

	response, err := r.aiClient.GenerateWithTools(ctx, aiData, tools, func(ctx context.Context, name string, args map[string]any) (string, error) {
		r.sendToolNotice(ctx, msg, name, args)
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

func (r *Router) sendToolNotice(ctx context.Context, msg Message, name string, args map[string]any) {
	if !r.threadedToolNoticesEnabled(msg.Metadata.From) {
		return
	}

	extra := make(map[string]any, len(msg.Metadata.Extra)+1)
	for k, v := range msg.Metadata.Extra {
		extra[k] = v
	}
	extra["msgtype"] = "notice"

	r.sendReply(ctx, msg.Metadata.From, fmt.Sprintf("> Using tool: `%s%s`", name, formatToolArgs(args)), msg.Metadata.Subject, extra)
}

func (r *Router) threadedToolNoticesEnabled(sourceID string) bool {
	r.mu.RLock()
	addr, ok := r.addresses[sourceID]
	r.mu.RUnlock()
	if !ok || r.app == nil || !strings.EqualFold(addr.System, "matrix") {
		return false
	}

	return r.app.Config.Matrix[addr.InstanceName].ThreadedTools
}

func formatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	argList := make([]string, 0, len(args))
	for k, v := range args {
		n := fmt.Sprintf("%v", v)
		if len(n) > maxToolArgLengthLog {
			n = fmt.Sprintf("%q", n[:maxToolArgLengthLog]+"...")
		} else {
			n = fmt.Sprintf("%q", n)
		}
		argList = append(argList, fmt.Sprintf("%s: %v", k, n))
	}

	return "(" + strings.Join(argList, ", ") + ")"
}
