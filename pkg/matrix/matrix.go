package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/jovandeginste/spark-personal-assistant/personas"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

type MatrixConfig struct {
	SourceID     uint64
	EF           app.EntryFilter
	AIData       *app.AIData
	AIClient     ai.Client
	Client       *mautrix.Client
	App          *app.App
	CryptoHelper *cryptohelper.CryptoHelper

	msges chan msg
}

type msg struct {
	RoomID  id.RoomID
	Content event.MessageEventContent
}

func (mc *MatrixConfig) ConfigureSyncer() {
	syncer := mc.Client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		mc.AIData.CleanHistory()
		body := evt.Content.AsMessage().Body

		mc.App.Logger().Info(
			"Received message",
			"sender", evt.Sender.String(),
			"room_id", evt.RoomID.String(),
			"type", evt.Type.String(),
			"id", evt.ID.String(),
			"timestamp", time.Unix(0, evt.Timestamp*int64(time.Millisecond)),
			"body", body,
			"history", len(mc.AIData.ChatHistory),
		)

		if err := mc.Client.MarkRead(context.Background(), evt.RoomID, evt.ID); err != nil {
			mc.App.Logger().Error("Failed to mark message as read", "error", err)
		}

		if evt.Sender.String() == mc.Client.UserID.String() {
			return
		}

		knownUser := slices.Contains(mc.App.Config.Matrix.Users, evt.Sender.String())
		if !knownUser {
			mc.App.Logger().Info("Unknown user - skipping", "user_id", evt.Sender.String())
			return
		}

		if err := mc.sendResponse(evt.RoomID, evt.Sender, body); err != nil {
			mc.sendNotice(evt.RoomID, "Failed to send response: "+err.Error())
			mc.App.Logger().Error("Failed to send response", "error", err)
		}
	})

	syncer.OnEventType(event.StateMember, mc.syncFunc)
}

func (mc *MatrixConfig) sendResponse(roomID id.RoomID, sender id.UserID, input string) error {
	mc.Client.UserTyping(context.Background(), roomID, true, 60*time.Second)
	defer func() {
		mc.Client.UserTyping(context.Background(), roomID, false, 0)
	}()

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return nil
	}

	result, err := mc.parseInput(input)
	if err != nil {
		return err
	}

	if result != "" {
		mc.sendMessage(roomID, result)
		return nil
	}

	result, err = mc.calculateResponse(roomID, sender, input)
	if err != nil {
		return err
	}

	mc.sendMessage(roomID, result)
	return nil
}

func (mc *MatrixConfig) performTasks(roomID id.RoomID) error {
	src, err := mc.App.FindSourceByName(mc.App.Config.Database.InternalSource)
	if err != nil {
		return err
	}

	d, err := mc.AIClient.GeneratePrompt(context.Background(), ai.PromptTask, mc.AIData)
	if err != nil {
		return fmt.Errorf("could not generate tasks: %w", err)
	}

	d = strings.Replace(d, "```json", "", 1)
	d = strings.Replace(d, "```", "", 1)

	mc.App.Logger().Info("Executing tasks", "tasks", d)

	var entries []entry

	if err := json.Unmarshal([]byte(d), &entries); err != nil {
		return err
	}

	for _, e := range entries {
		e.Execute(mc, roomID, src)
	}

	if err := mc.AIData.UpdateEntries(mc.App); err != nil {
		return err
	}

	return nil
}

func (mc *MatrixConfig) parseInput(input string) (string, error) {
	cmd := strings.SplitN(input, " ", 3)

	switch cmd[0] {
	case "reset":
		defer mc.AIData.ResetHistory()
		return "Resetting chat history...", nil
	case "shutdown":
		go func() {
			time.Sleep(5 * time.Second)
			os.Exit(1)
		}()

		return "Shutting down...", nil
	case "today":
		f := &app.EntryFilter{DaysBack: 1, DaysAhead: 1}

		entries, err := mc.App.CurrentEntries(f)
		if err != nil {
			return "", err
		}

		b := bytes.Buffer{}
		entries.PrintTo(&b, true)

		return b.String(), nil
	case "switch":
		if len(cmd) < 2 {
			pDir, err := personas.FS().ReadDir(".")
			if err != nil {
				return "", err
			}

			list := "All personas:\n"
			for _, p := range pDir {
				n := p.Name()
				n = strings.TrimSuffix(n, ".md")
				list += fmt.Sprintf("- %s\n", n)
			}

			return list, nil
		}

		mc.App.SetPersona(cmd[1])
		mc.Greet()

		return "Switched to persona: " + mc.App.Config.Assistant.Name, nil
	case "summarize":
		if len(cmd) < 2 {
			return "", nil
		}

		mc.AIData.ResetHistory()

		switch cmd[1] {
		case "full":
			return mc.AIClient.GeneratePrompt(context.Background(), ai.PromptFull, mc.AIData)
		case "week":
			return mc.AIClient.GeneratePrompt(context.Background(), ai.PromptWeek, mc.AIData)
		case "today":
			return mc.AIClient.GeneratePrompt(context.Background(), ai.PromptToday, mc.AIData)
		default:
			return "", nil
		}
	case "ping":
		return "*pong back*", nil
	default:
		return "", nil
	}
}

func (mc *MatrixConfig) checkIfTask(input string) (bool, string) {
	if strings.HasPrefix(input, "!") {
		input = strings.TrimLeft(input, "!")
		return true, input
	}

	return false, input
}

func (mc *MatrixConfig) calculateResponse(roomID id.RoomID, sender id.UserID, input string) (string, error) {
	mc.App.Logger().Info("Parsing question...", "sender", sender)
	mc.AIData.EmployerQuestion = []string{fmt.Sprintf("Sender: %s", sender), input}

	if err := mc.AIData.UpdateEntries(mc.App); err != nil {
		return "", err
	}

	isTask, _ := mc.checkIfTask(input)
	if isTask {
		mc.sendNotice(roomID, "Calculating tasks...")

		if err := mc.performTasks(roomID); err != nil {
			return "", fmt.Errorf("could not perform task: %w", err)
		}
	}

	mc.sendNotice(roomID, "Calculating response...")

	md, err := mc.AIClient.GeneratePrompt(context.Background(), ai.PromptCustom, mc.AIData)
	if err != nil {
		return "", fmt.Errorf("could not calculate response: %w", err)
	}

	mc.AIData.AddChatHistory("user", input)
	mc.AIData.AddChatHistory("assistant", md)

	return md, nil
}

func (mc *MatrixConfig) syncFunc(ctx context.Context, evt *event.Event) {
	if evt.GetStateKey() != mc.Client.UserID.String() || evt.Content.AsMember().Membership != event.MembershipInvite {
		return
	}

	if _, err := mc.Client.JoinRoomByID(ctx, evt.RoomID); err != nil {
		mc.App.Logger().Error(
			"Failed to join room after invite",
			"room_id", evt.RoomID.String(),
			"inviter", evt.Sender.String(),
			"error", err,
		)
		return
	}

	mc.App.Logger().Info(
		"Joined room after invite",
		"room_id", evt.RoomID.String(),
		"inviter", evt.Sender.String(),
	)
}

func (mc *MatrixConfig) InitClient() error {
	client, err := mautrix.NewClient(mc.App.Config.Matrix.Homeserver, "", "")
	if err != nil {
		return err
	}

	mc.Client = client

	return nil
}

func (mc *MatrixConfig) ConfigureCryptoHelper() error {
	cryptoHelper, err := cryptohelper.NewCryptoHelper(mc.Client, []byte("meow"), mc.App.Config.Matrix.Database)
	if err != nil {
		return err
	}

	cryptoHelper.LoginAs = &mautrix.ReqLogin{
		Type:       mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: mc.App.Config.Matrix.Username},
		Password:   mc.App.Config.Matrix.Password,
	}

	if err := cryptoHelper.Init(context.TODO()); err != nil {
		return err
	}

	mc.CryptoHelper = cryptoHelper
	mc.Client.Crypto = cryptoHelper

	return nil
}

func (mc *MatrixConfig) InitChat() {
	mc.msges = make(chan msg, 10)

	go func() {
		for msg := range mc.msges {
			resp, err := mc.Client.SendMessageEvent(context.Background(), msg.RoomID, event.EventMessage, msg.Content)
			if err != nil {
				mc.App.Logger().Error("Error sending event", "error", err)
				continue
			}

			mc.App.Logger().Info("Event sent", "event_id", resp.EventID.String())
		}
	}()
}

func (mc *MatrixConfig) send(roomID id.RoomID, msgType event.MessageType, text string) {
	content := format.RenderMarkdown(text, true, true)
	content.MsgType = msgType

	mc.msges <- msg{RoomID: roomID, Content: content}
}

func (mc *MatrixConfig) sendNotice(roomID id.RoomID, text string) {
	mc.App.Logger().Info(text)
	mc.send(roomID, event.MsgNotice, text)
}

func (mc *MatrixConfig) sendMessage(roomID id.RoomID, text string) {
	mc.send(roomID, event.MsgText, text)
}

func (mc *MatrixConfig) Greet() error {
	if err := mc.Client.SetDisplayName(context.Background(), mc.App.Config.Assistant.Name); err != nil {
		mc.App.Logger().Error("Failed to set display name", "error", err)
	}

	mc.send(mc.DefaultRoomID(), event.MsgEmote, "reporting for duty")

	return nil
}

func (mc *MatrixConfig) DefaultRoomID() id.RoomID {
	return id.RoomID(mc.App.Config.Matrix.RoomID)
}
