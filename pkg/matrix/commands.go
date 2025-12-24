package matrix

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/personas"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"maunium.net/go/mautrix/id"
)

func (mc *MatrixConfig) performCommandReset() (string, error) {
	mc.AIData.ResetHistory()
	return "History is clear", nil
}

func (mc *MatrixConfig) performCommandUpdate(roomID id.RoomID) (string, error) {
	go func() {
		if err := mc.App.UpdateSources(); err != nil {
			mc.sendMessage(roomID, "Something went wrong while updating sources:\n\n> "+strings.ReplaceAll(err.Error(), "\n", "\n> "))
		}

		mc.sendNotice(roomID, "Sources were updated.")
	}()

	return "Updating all sources", nil
}

func (mc *MatrixConfig) performCommandShutdown() (string, error) {
	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}()

	return "Shutting down...", nil
}

func (mc *MatrixConfig) performCommandShowToday() (string, error) {
	f := &app.EntryFilter{DaysBack: 1, DaysAhead: 1}

	entries, err := mc.App.CurrentEntries(f)
	if err != nil {
		return "", err
	}

	b := bytes.Buffer{}
	entries.PrintTo(&b, true)

	return b.String(), nil
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

	if err := mc.AIData.UpdateEntries(mc.App); err != nil {
		return "", err
	}

	switch name {
	case "full":
		return mc.AIClient.GeneratePrompt(context.Background(), ai.PromptFull, mc.AIData)
	case "week":
		return mc.AIClient.GeneratePrompt(context.Background(), ai.PromptWeek, mc.AIData)
	case "today":
		return mc.AIClient.GeneratePrompt(context.Background(), ai.PromptToday, mc.AIData)
	default:
		return "", nil
	}
}

func (mc *MatrixConfig) performCommandPing() (string, error) {
	return "*pong back*", nil
}

func (mc *MatrixConfig) parseInput(input string, roomID id.RoomID) (string, error) {
	cmd := strings.SplitN(input, " ", 3)

	switch cmd[0] {
	case "reset":
		return mc.performCommandReset()
	case "update":
		return mc.performCommandUpdate(roomID)
	case "shutdown":
		return mc.performCommandShutdown()
	case "today":
		return mc.performCommandShowToday()
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
	case "ping":
		return mc.performCommandPing()
	default:
		return "", nil
	}
}
