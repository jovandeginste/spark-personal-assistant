package router

import (
	"context"
	"fmt"
	"strings"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
)

type Router struct {
	aiClient ai.Client
	app      *app.App

	// Map of backend channels
	backends map[string]Backend
}

type Message struct {
	Source    string
	Target    string
	Content   string
	ReplyToID string
	// Any metadata context (like room ID, user ID, sender info)
	Metadata map[string]any
}

type Backend struct {
	Incoming chan Message
	Outgoing chan Message
}

func NewRouter(aiClient ai.Client, a *app.App) *Router {
	return &Router{
		aiClient: aiClient,
		app:      a,
		backends: make(map[string]Backend),
	}
}

// RegisterBackend registers a new backend with the router
func (r *Router) RegisterBackend(name string, incoming chan Message, outgoing chan Message) {
	r.backends[name] = Backend{
		Incoming: incoming,
		Outgoing: outgoing,
	}
}

// SubmitMessage allows interfaces to send messages to the router
func (r *Router) SubmitMessage(msg Message) {
	if backend, ok := r.backends[msg.Source]; ok {
		backend.Incoming <- msg
	}
}

// Start begins processing messages in the background
func (r *Router) Start(ctx context.Context) {
	for name, backend := range r.backends {
		go r.processBackend(ctx, name, backend)
	}
}

func (r *Router) processBackend(ctx context.Context, name string, backend Backend) {
	for {
		select {
		case <-ctx.Done():
			r.app.Logger().Info("Router backend stopped", "name", name)
			return
		case msg := <-backend.Incoming:
			r.handleMessage(ctx, msg)
		}
	}
}

func (r *Router) handleMessage(ctx context.Context, msg Message) {
	r.app.Logger().Info("Routing message", "source", msg.Source, "target", msg.Target)

	if msg.Target == "ai" {
		r.routeToAI(ctx, msg)
	} else if backend, ok := r.backends[msg.Target]; ok {
		backend.Outgoing <- msg
	} else {
		r.app.Logger().Warn("Unknown target backend", "target", msg.Target)
	}
}

func (r *Router) routeToAI(ctx context.Context, msg Message) {
	// Simple route using GenerateWithTools to have parity with Matrix handler
	roomID := ""
	if msg.Metadata != nil {
		if rID, ok := msg.Metadata["room_id"].(string); ok {
			roomID = rID
		}
	}

	tools, err := r.app.GetMCPTools(ctx, roomID)
	if err != nil {
		r.app.Logger().Error("Failed to get MCP tools", "error", err)
	}

	backendNames := make([]string, 0, len(r.backends))
	for name := range r.backends {
		backendNames = append(backendNames, name)
	}

	// Add special purpose tool for AI to send messages
	tools = append(tools, ai.Tool{
		Name:        "send_message",
		Description: fmt.Sprintf("Send a message to a specific backend (%s). Important: Messages sent by the AI will be logged to the source channel so the user knows what was sent.", strings.Join(backendNames, ", ")),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "The backend to send the message to",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content of the message",
				},
				"address": map[string]any{
					"type":        "string",
					"description": "The intended target address (e.g., email address for imap)",
				},
				"subject": map[string]any{
					"type":        "string",
					"description": "The intended subject (e.g., email subject for imap)",
				},
			},
			"required": []any{"target", "content"},
		},
	})

	r.app.Logger().Info("Using tools in router", "count", len(tools))

	aiData, err := r.app.BuildData()
	if err != nil {
		r.app.Logger().Error("Failed to build AI data", "error", err)
	}
	if aiData != nil {
		aiData.EmployerQuestion = []string{msg.Content}
	}

	response, err := r.aiClient.GenerateWithTools(ctx, aiData, tools, func(ctx context.Context, name string, args map[string]any) (string, error) {
		if name == "send_message" {
			target, _ := args["target"].(string)
			content, _ := args["content"].(string)
			address, _ := args["address"].(string)
			subject, _ := args["subject"].(string)

			// Send to intended target
			if backend, ok := r.backends[target]; ok {
				outMsg := Message{
					Source:   "ai",
					Target:   target,
					Content:  content,
					Metadata: make(map[string]any),
				}
				for k, v := range msg.Metadata {
					outMsg.Metadata[k] = v
				}
				if address != "" {
					outMsg.Metadata["target_address"] = address
				}
				if subject != "" {
					outMsg.Metadata["target_subject"] = subject
				}
				backend.Outgoing <- outMsg
			}

			// Log to source channel
			if sourceBackend, ok := r.backends[msg.Source]; ok && msg.Source != target {
				metaStr := ""
				if address != "" {
					metaStr += "\nTo: " + address
				}
				if subject != "" {
					metaStr += "\nSubject: " + subject
				}
				sourceBackend.Outgoing <- Message{
					Source:   "ai",
					Target:   msg.Source,
					Content:  "I have sent the following message to " + target + metaStr + ":\n\n" + content,
					Metadata: msg.Metadata,
				}
			}
			return "Message sent to " + target + " successfully.", nil
		}
		return r.app.ExecuteMCPTool(ctx, name, args, roomID)
	}, nil)
	if err != nil {
		r.app.Logger().Error("Failed to get AI response", "error", err)
		if backend, ok := r.backends[msg.Source]; ok {
			backend.Outgoing <- Message{
				Source:   "ai",
				Target:   msg.Source,
				Content:  "Sorry, I encountered an error processing your request.",
				Metadata: msg.Metadata,
			}
		}
		return
	}

	if backend, ok := r.backends[msg.Source]; ok {
		backend.Outgoing <- Message{
			Source:   "ai",
			Target:   msg.Source,
			Content:  response,
			Metadata: msg.Metadata,
		}
	}
}
