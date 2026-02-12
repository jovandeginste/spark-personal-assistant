package matrix

import (
	"context"
	"database/sql"

	// "encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/glebarez/go-sqlite"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

func init() {
	sql.Register("sqlite3-fk-wal", &sqlite.Driver{})
}

type MatrixConfig struct {
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

		// Handle file attachments
		if evt.Content.AsMessage().MsgType != event.MsgText && evt.Content.AsMessage().MsgType != event.MsgEmote {
			mc.App.Logger().Info("Received attachment", "type", evt.Content.AsMessage().MsgType, "filename", evt.Content.AsMessage().Body)

			fileURL := evt.Content.AsMessage().URL
			if fileURL == "" {
				fileURL = evt.Content.AsMessage().File.URL
			}

			if fileURL != "" {
				uri, err := fileURL.Parse()
				if err != nil {
					mc.App.Logger().Error("Failed to parse file URL", "error", err)
					return
				}

				file, err := mc.Client.DownloadBytes(ctx, uri)
				if err != nil {
					mc.App.Logger().Error("Failed to download file", "error", err)
					mc.sendNotice(evt.RoomID, fmt.Sprintf("Failed to download file %s: %v", evt.Content.AsMessage().Body, err))
					return
				}

				aiURI, err := mc.AIClient.UploadFile(ctx, evt.Content.AsMessage().Body, file, evt.Content.AsMessage().Info.MimeType)
				if err != nil {
					mc.App.Logger().Error("Failed to upload file to AI", "error", err)
					mc.sendNotice(evt.RoomID, fmt.Sprintf("Failed to upload file %s to AI: %v", evt.Content.AsMessage().Body, err))
					return
				}

				mc.AIData.FileURIs = append(mc.AIData.FileURIs, aiURI)
				mc.sendNotice(evt.RoomID, fmt.Sprintf("Received file %s and uploaded to AI", evt.Content.AsMessage().Body))
			} else {
				mc.App.Logger().Warn("Received attachment but no URL found", "body", evt.Content.AsMessage().Body)
			}
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

func (mc *MatrixConfig) calculateResponse(roomID id.RoomID, sender id.UserID, input string) (string, error) {
	mc.App.Logger().Info("Parsing question...", "sender", sender)
	mc.AIData.EmployerQuestion = []string{fmt.Sprintf("Sender: %s", sender), input}

	mc.sendNotice(roomID, "Calculating response...")

	tools, err := mc.App.GetMCPTools(context.Background())
	if err != nil {
		mc.App.Logger().Error("Failed to get MCP tools", "error", err)
	}

	mc.App.Logger().Info("Using tools", "count", len(tools))

	md, err := mc.AIClient.GenerateWithTools(context.Background(), ai.PromptCustom, mc.AIData, tools, func(ctx context.Context, name string, args map[string]any) (string, error) {
		return mc.App.ExecuteMCPTool(ctx, name, args)
	}, mc.AIData.FileURIs)
	if err != nil {
		return "", fmt.Errorf("could not calculate response: %w", err)
	}

	mc.AIData.AddChatHistory("user", input)
	mc.AIData.AddChatHistory("assistant", md)

	// Clean up file URIs after they've been used
	mc.AIData.FileURIs = nil

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
	cryptoHelper, err := cryptohelper.NewCryptoHelper(mc.Client, []byte("meow"), mc.App.Config.Matrix.CryptoStore)
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
