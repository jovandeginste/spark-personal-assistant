package matrix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/personas"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"maunium.net/go/mautrix"
)

func (mc *MatrixConfig) performCommandResetRecovery() (string, error) {
	if mc.CryptoHelper == nil {
		return "", errors.New("crypto helper is not initialized")
	}

	mach := mc.CryptoHelper.Machine()
	recoveryKey, _, err := mach.GenerateAndUploadCrossSigningKeys(context.Background(), func(uiResp *mautrix.RespUserInteractive) any {
		type ReqUIAuthLogin struct {
			mautrix.BaseAuthData
			Identifier mautrix.UserIdentifier `json:"identifier"`
			Password   string                 `json:"password"`
		}

		return &ReqUIAuthLogin{
			BaseAuthData: mautrix.BaseAuthData{
				Type:    mautrix.AuthTypePassword,
				Session: uiResp.Session,
			},
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: mc.App.Config.Matrix.Username, // Use Username instead of UserID (typically "@user:hs.tld") for identifier
			},
			Password: mc.App.Config.Matrix.Password,
		}
	}, "")
	if err != nil {
		return "", fmt.Errorf("failed to reset recovery key: %w", err)
	}

	if err = mach.SignOwnDevice(context.Background(), mach.OwnIdentity()); err != nil {
		return "", fmt.Errorf("failed to sign own device: %w", err)
	}
	if err = mach.SignOwnMasterKey(context.Background()); err != nil {
		return "", fmt.Errorf("failed to sign own master key: %w", err)
	}

	return fmt.Sprintf("New recovery key generated:\n\n`%s`\n\nPlease save this key securely.", recoveryKey), nil
}

func (mc *MatrixConfig) performCommandReset() (string, error) {
	mc.AIData.ResetHistory()
	return "History is clear", nil
}

func (mc *MatrixConfig) performCommandShutdown() (string, error) {
	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}()

	return "Shutting down...", nil
}

func (mc *MatrixConfig) performCommandListPersona() (string, error) {
	pDir, err := personas.FS().ReadDir(".")
	if err != nil {
		return "", err
	}

	var list strings.Builder

	list.WriteString("All personas:\n")

	for _, p := range pDir {
		n := p.Name()
		n = strings.TrimSuffix(n, ".md")
		list.WriteString(fmt.Sprintf("- %s\n", n))
	}

	return list.String(), nil
}

func (mc *MatrixConfig) performCommandSwitchPersona(name string) (string, error) {
	mc.App.SetPersona(name)
	mc.Greet()

	return "Switched to persona: " + mc.App.Config.Assistant.Name, nil
}

func (mc *MatrixConfig) performCommandSummarize(name string) (string, error) {
	mc.AIData.ResetHistory()

	var p ai.Prompt

	switch name {
	case "full":
		p = ai.PromptFull
	case "week":
		p = ai.PromptWeek
	case "today":
		p = ai.PromptToday
	default:
		// Assume it is a custom prompt
		mc.AIData.EmployerQuestion = []string{"Summarize the events based on the prompt: " + name}
		p = ai.PromptCustom
	}

	tools, err := mc.App.GetMCPTools(context.Background())
	if err != nil {
		mc.App.Logger().Error("Failed to get MCP tools", "error", err)
	}

	return mc.AIClient.GenerateWithTools(context.Background(), p, mc.AIData, tools, func(ctx context.Context, name string, args map[string]any) (string, error) {
		return mc.App.ExecuteMCPTool(ctx, name, args)
	}, mc.AIData.FileURIs)
}

func (mc *MatrixConfig) performCommandUpdate() (string, error) {
	mc.sendNotice(mc.DefaultRoomID(), "Updating MCP servers...")

	results := mc.App.UpdateMCPServers(context.Background())

	var out strings.Builder
	out.WriteString("Update results:\n")
	for name, res := range results {
		out.WriteString(fmt.Sprintf("- %s: %s\n", name, res))
	}

	return out.String(), nil
}

func (mc *MatrixConfig) performCommandPing() (string, error) {
	return "*pong back*", nil
}

func (mc *MatrixConfig) parseInput(input string) (string, error) {
	if !strings.HasPrefix(input, "!") {
		return "", nil
	}

	input = strings.TrimPrefix(input, "!")
	cmd := strings.SplitN(input, " ", 3)

	switch cmd[0] {
	case "reset":
		return mc.performCommandReset()
	case "shutdown":
		return mc.performCommandShutdown()
	case "switch":
		if len(cmd) < 2 {
			return mc.performCommandListPersona()
		}

		return mc.performCommandSwitchPersona(cmd[1])
	case "summarize":
		if len(cmd) < 2 {
			return "", nil
		}

		return mc.performCommandSummarize(cmd[1])
	case "update":
		return mc.performCommandUpdate()
	case "ping":
		return mc.performCommandPing()
	case "recovery":
		return mc.performCommandResetRecovery()
	default:
		return "", nil
	}
}
