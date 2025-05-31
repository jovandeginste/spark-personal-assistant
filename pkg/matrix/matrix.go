package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	_ "github.com/mattn/go-sqlite3"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

type MatrixConfig struct {
	Database string
	Source   string

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
		body := evt.Content.AsMessage().Body

		mc.App.Logger().Info(
			"Received message",
			"sender", evt.Sender.String(),
			"type", evt.Type.String(),
			"id", evt.ID.String(),
			"body", body,
		)

		if err := mc.Client.MarkRead(context.Background(), evt.RoomID, evt.ID); err != nil {
			mc.App.Logger().Error("Failed to mark message as read", "error", err)
		}

		if evt.Sender.String() == mc.Client.UserID.String() {
			return
		}

		if err := mc.sendResponse(evt.RoomID, evt.Sender, body); err != nil {
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

	r, err := mc.calculateResponse(roomID, sender, input)
	if err != nil {
		mc.App.Logger().Error("Failed to calculate response", "error", err)
		r = fmt.Sprintf("Error: %s", err)
	}

	mc.sendMessage(roomID, r)

	return nil
}

func (mc *MatrixConfig) performTasks(roomID id.RoomID) error {
	src, err := mc.App.FindSourceByName(mc.Source)
	if err != nil {
		return err
	}

	d, err := mc.AIClient.GeneratePrompt(context.Background(), ai.PromptTask, mc.AIData)
	if err != nil {
		return fmt.Errorf("could not generate tasks: %w", err)
	}

	d = strings.Replace(d, "```json", "", 1)
	d = strings.Replace(d, "```", "", 1)

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

func (mc *MatrixConfig) calculateResponse(roomID id.RoomID, sender id.UserID, input string) (string, error) {
	input = strings.TrimSpace(input)
	switch input {
	case "":
		return "", nil
	case "ping":
		return "*pong back*", nil
	}

	mc.App.Logger().Info("Parsing question...", "sender", sender)
	mc.AIData.EmployerQuestion = []string{fmt.Sprintf("Sender: %s", sender), input}

	if err := mc.AIData.UpdateEntries(mc.App); err != nil {
		return "", err
	}

	isTask, err := mc.AIClient.GeneratePrompt(context.Background(), ai.PromptCheckTask, mc.AIData.EmployerQuestion)
	if err != nil {
		return "", err
	}

	if isTask = strings.TrimSpace(isTask); isTask != "0" {
		mc.sendMessage(roomID, isTask)
		mc.App.Logger().Info("Calculating tasks...")

		err := mc.performTasks(roomID)
		if err != nil {
			return "", fmt.Errorf("could not perform task: %w", err)
		}
	}

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
	cryptoHelper, err := cryptohelper.NewCryptoHelper(mc.Client, []byte("meow"), mc.Database)
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

func (mc *MatrixConfig) sendMessage(roomID id.RoomID, text string) {
	content := format.RenderMarkdown(text, true, true)

	mc.msges <- msg{RoomID: roomID, Content: content}
}
