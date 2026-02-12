package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/iterator"
	"google.golang.org/genai"
)

type geminiClient struct {
	apiKey    string
	model     string
	assistant *AssistantConfig
	logger    *slog.Logger
}

func (c geminiClient) Logger() *slog.Logger {
	return c.logger
}

func (c geminiClient) APIKey() string {
	return c.apiKey
}

func (c geminiClient) Model() string {
	return c.model
}

func (c geminiClient) convertPrompt(p Prompt, data any) (*genai.Content, error) {
	prompt, err := p(c.assistant, data)
	if err != nil {
		return nil, err
	}

	c.Logger().Info("prompt size", "size", len(fmt.Sprintf("%+v", prompt)))

	parts := make([]*genai.Part, 0, len(prompt))

	for _, part := range prompt {
		parts = append(parts, &genai.Part{Text: part})
	}

	return genai.NewContentFromParts(parts, genai.RoleUser), nil
}

func (c geminiClient) GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error) {
	c.Logger().Info("Fetching result from AI...")

	prompt, err := c.convertPrompt(p, data)
	if err != nil {
		return "", err
	}

	c.Logger().Info("Prompt", "prompt", prompt)

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return "", err
	}

	config := &genai.GenerateContentConfig{}

	var result *genai.GenerateContentResponse

	for i := range 10 {
		result, err = client.Models.GenerateContent(ctx, c.model, []*genai.Content{prompt}, config)
		if err == nil {
			break
		}

		if i < 10 {
			c.Logger().Warn("Retrying in 30 seconds...", "error", err)
			time.Sleep(30 * time.Second)
			continue
		}

		c.Logger().Error("Failed to generate prompt", "error", err)

		return "", err
	}

	c.Logger().Info("Parsing results")

	for _, c := range result.Candidates {
		if len(c.Content.Parts) == 0 {
			continue
		}

		return c.Content.Parts[0].Text, nil
	}

	c.Logger().Info("No result")

	return "", nil
}

func (c geminiClient) GenerateWithTools(ctx context.Context, p Prompt, data any, tools []Tool, executor ToolExecutor, fileURIs []string) (string, error) {
	c.Logger().Info("Fetching result from AI with tools...")

	promptContent, err := c.convertPrompt(p, data)
	if err != nil {
		return "", err
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return "", err
	}

	config := &genai.GenerateContentConfig{
		Tools: c.convertTools(tools),
	}

	history := []*genai.Content{promptContent}
	if len(fileURIs) > 0 {
		files := make([]*genai.Part, 0, len(fileURIs))
		for _, uri := range fileURIs {
			files = append(files, &genai.Part{FileData: &genai.FileData{FileURI: uri}})
		}
		history = append(history, genai.NewContentFromParts(files, genai.RoleUser))
	}

	for i := range MaxToolCalls {
		result, err := client.Models.GenerateContent(ctx, c.model, history, config)
		if err != nil {
			c.Logger().Warn("Error generating content", "error", err, "attempt", i)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(result.Candidates) == 0 || result.Candidates[0].Content == nil {
			return "", errors.New("no candidates returned")
		}

		candidate := result.Candidates[0]
		history = append(history, candidate.Content)

		toolResponses, hasToolCall := c.executeToolCalls(ctx, candidate, executor)

		if !hasToolCall {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					return part.Text, nil
				}
			}

			return "", nil
		}

		history = append(history, genai.NewContentFromParts(toolResponses, genai.RoleModel))
	}

	return "", errors.New("reached maximum iterations with tool calls")
}

func (c geminiClient) executeToolCalls(ctx context.Context, candidate *genai.Candidate, executor ToolExecutor) ([]*genai.Part, bool) {
	hasToolCall := false
	toolResponses := []*genai.Part{}

	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			hasToolCall = true

			c.Logger().Info("Tool call received", "tool", part.FunctionCall.Name, "args", part.FunctionCall.Args)

			resp, err := executor(ctx, part.FunctionCall.Name, part.FunctionCall.Args)
			if err != nil {
				c.Logger().Error("Tool execution failed", "error", err)
				resp = fmt.Errorf("tool execution failed: %w", err).Error()
			}

			toolResponses = append(toolResponses, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     part.FunctionCall.Name,
					Response: map[string]any{"result": resp},
				},
			})
		}
	}

	return toolResponses, hasToolCall
}

func (c geminiClient) convertTools(tools []Tool) []*genai.Tool {
	genaiTools := make([]*genai.Tool, 0, len(tools))
	for _, t := range tools {
		schema := c.convertToolSchema(t)

		genaiTools = append(genaiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  schema,
				},
			},
		})
	}
	return genaiTools
}

func (c geminiClient) convertToolSchema(t Tool) *genai.Schema {
	if s, ok := t.InputSchema.(*genai.Schema); ok {
		return s
	}

	b, err := json.Marshal(t.InputSchema)
	if err != nil {
		c.Logger().Error("Failed to marshal tool schema", "tool", t.Name, "error", err)
		return nil
	}

	var rawSchema any
	if err := json.Unmarshal(b, &rawSchema); err != nil {
		c.Logger().Error("Failed to unmarshal to raw schema", "tool", t.Name, "error", err)
		return nil
	}

	sanitizeSchema(rawSchema)

	b, err = json.Marshal(rawSchema)
	if err != nil {
		c.Logger().Error("Failed to marshal sanitized schema", "tool", t.Name, "error", err)
		return nil
	}

	var schema *genai.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		c.Logger().Error("Failed to unmarshal tool schema to genai.Schema", "tool", t.Name, "error", err)
		return nil
	}

	return schema
}

func sanitizeSchema(data any) {
	switch v := data.(type) {
	case map[string]any:
		if enumVal, ok := v["enum"]; ok {
			if enumList, ok := enumVal.([]any); ok {
				newEnum := make([]string, 0, len(enumList))
				for _, val := range enumList {
					newEnum = append(newEnum, fmt.Sprintf("%v", val))
				}

				v["enum"] = newEnum
				// Enums must be of type string for Gemini
				v["type"] = "string"
			}
		}

		for _, val := range v {
			sanitizeSchema(val)
		}
	case []any:
		for _, val := range v {
			sanitizeSchema(val)
		}
	}
}

func (c geminiClient) UploadFile(ctx context.Context, name string, content []byte, mimeType string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return "", err
	}

	// Changed from client.Models.UploadFile to client.Files.Upload
	f, err := client.Files.Upload(ctx, bytes.NewReader(content), &genai.UploadFileConfig{
		DisplayName: name,
		MIMEType:    mimeType,
	})
	if err != nil {
		return "", err
	}

	return f.URI, nil
}

func (c geminiClient) ListFiles(ctx context.Context) ([]string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return nil, err
	}

	var uris []string
	// Changed from client.Models.ListFiles to client.Files.List
	iter, err := client.Files.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for {
		page, err := iter.Next(ctx)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, f := range page.Items {
			uris = append(uris, f.URI)
		}
	}

	return uris, nil
}

func (c geminiClient) DeleteFile(ctx context.Context, name string) error {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return err
	}

	// Changed from client.Models.DeleteFile to client.Files.Delete
	_, err = client.Files.Delete(ctx, name, nil)
	return err
}
