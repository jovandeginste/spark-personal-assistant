package router

import (
	"context"
	"log/slog"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
)

type Router struct {
	aiClient ai.Client

	// Channels for routing messages
	incoming chan Message
	outgoing chan Message
}

type Message struct {
	Source    string
	Target    string
	Content   string
	ReplyToID string
	// Any metadata context (like room ID, user ID, sender info)
	Metadata map[string]any
}

func NewRouter(aiClient ai.Client) *Router {
	return &Router{
		aiClient: aiClient,
		incoming: make(chan Message, 100),
		outgoing: make(chan Message, 100),
	}
}

// Start begins processing messages in the background
func (r *Router) Start(ctx context.Context) {
	go r.processIncoming(ctx)
}

// SubmitMessage allows interfaces (Matrix, Web, IMAP) to send messages to the router
func (r *Router) SubmitMessage(msg Message) {
	r.incoming <- msg
}

// OutgoingMessages returns a channel of messages intended for the interfaces
func (r *Router) OutgoingMessages() <-chan Message {
	return r.outgoing
}

func (r *Router) processIncoming(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("Router stopped")
			return
		case msg := <-r.incoming:
			r.handleMessage(ctx, msg)
		}
	}
}

func (r *Router) handleMessage(ctx context.Context, msg Message) {
	slog.Info("Routing message", "source", msg.Source, "target", msg.Target)

	if msg.Target == "ai" {
		// Example implementation of asking AI and replying back to source
		r.routeToAI(ctx, msg)
	} else {
		// Directly forward to the target interface
		r.outgoing <- msg
	}
}

func (r *Router) routeToAI(ctx context.Context, msg Message) {
	// Simple route using GeneratePrompt
	response, err := r.aiClient.GeneratePrompt(ctx, msg.Content)
	if err != nil {
		slog.Error("Failed to get AI response", "error", err)
		r.outgoing <- Message{
			Source:   "ai",
			Target:   msg.Source,
			Content:  "Sorry, I encountered an error processing your request.",
			Metadata: msg.Metadata,
		}
		return
	}

	r.outgoing <- Message{
		Source:   "ai",
		Target:   msg.Source,
		Content:  response,
		Metadata: msg.Metadata,
	}
}
