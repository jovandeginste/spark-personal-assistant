// Copyright (C) 2017 Tulir Asokan
// Copyright (C) 2018-2020 Luca Weiss
// Copyright (C) 2023 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

type matrixConfig struct {
	homeserver string
	username   string
	password   string
	database   string
	source     string

	sourceID     uint64
	ef           app.EntryFilter
	aiData       *app.AIData
	aiClient     ai.Client
	client       *mautrix.Client
	app          *app.App
	cryptoHelper *cryptohelper.CryptoHelper

	msges chan msg
}

type msg struct {
	RoomID  id.RoomID
	Content event.MessageEventContent
}

func (c *cli) matrixChatCmd() *cobra.Command {
	mc := matrixConfig{app: c.app}

	cmd := &cobra.Command{
		Use:     "matrix",
		Short:   "Start Matrix client with Spark",
		Example: "spark matrix",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := mc.app.FindSourceByName(mc.source)
			if err != nil {
				return err
			}

			mc.sourceID = src.ID

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant)
			if err != nil {
				return err
			}

			mc.aiClient = aiClient
			aiData, err := c.app.BuildData(&mc.ef)
			if err != nil {
				return err
			}

			mc.aiData = aiData

			if err := mc.initClient(); err != nil {
				return err
			}

			mc.configureSyncer()

			if err := mc.configureCryptoHelper(); err != nil {
				return err
			}

			mc.initChat()
			c.app.Logger().Info("Now running", "client_id", mc.client.UserID.String())

			if err := mc.client.SyncWithContext(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
				c.app.Logger().Error("Failed to sync", "error", err)
			}

			return mc.cryptoHelper.Close()
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&c.app.Config.AssistantFileCLI, "persona", "", "persona")
	cmd.Flags().StringVar(&mc.homeserver, "homeserver", "", "Matrix homeserver")
	cmd.Flags().StringVar(&mc.username, "username", "", "Matrix username localpart")
	cmd.Flags().StringVar(&mc.password, "password", "", "Matrix password")
	cmd.Flags().StringVar(&mc.database, "database", "mautrix-example.db", "SQLite database path")
	cmd.Flags().StringVar(&mc.source, "source", "custom", "Name of the source to use")
	cmd.Flags().UintVarP(&mc.ef.DaysBack, "days-back", "b", 30, "Number of days in the past to include")
	cmd.Flags().UintVarP(&mc.ef.DaysAhead, "days-ahead", "a", 90, "Number of days in the future to include")

	return cmd
}

func (mc *matrixConfig) configureSyncer() {
	syncer := mc.client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		body := evt.Content.AsMessage().Body

		mc.app.Logger().Info(
			"Received message",
			"sender", evt.Sender.String(),
			"type", evt.Type.String(),
			"id", evt.ID.String(),
			"body", body,
		)

		if err := mc.client.MarkRead(context.Background(), evt.RoomID, evt.ID); err != nil {
			mc.app.Logger().Error("Failed to mark message as read", "error", err)
		}

		if evt.Sender.String() == mc.client.UserID.String() {
			return
		}

		if err := mc.sendResponse(evt.RoomID, evt.Sender, body); err != nil {
			mc.app.Logger().Error("Failed to send response", "error", err)
		}
	})

	syncer.OnEventType(event.StateMember, mc.syncFunc)
}

func (mc *matrixConfig) sendResponse(roomID id.RoomID, sender id.UserID, input string) error {
	mc.client.UserTyping(context.Background(), roomID, true, 60*time.Second)
	defer func() {
		mc.client.UserTyping(context.Background(), roomID, false, 0)
	}()

	r, err := mc.calculateResponse(roomID, sender, input)
	if err != nil {
		mc.app.Logger().Error("Failed to calulate response", "error", err)
		r = fmt.Sprintf("Error: %s", err)
	}

	mc.sendMessage(roomID, r)

	return nil
}

type entry struct {
	Action      string `json:"action"`
	Date        string `json:"date"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Name        string `json:"name"`
}

func (e *entry) ToEntry() *data.Entry {
	de := &data.Entry{Summary: e.Summary}
	if err := de.SetDate(e.Date); err != nil {
		return nil
	}

	de.SetMetadata("description", e.Description)
	de.SetMetadata("person", e.Name)

	return de
}

func (mc *matrixConfig) performTasks(roomID id.RoomID, sender id.UserID, input string) error {
	src, err := mc.app.FindSourceByName(mc.source)
	if err != nil {
		return err
	}

	d, err := mc.aiClient.GeneratePrompt(context.Background(), ai.PromptTask, mc.aiData)
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

	if err := mc.aiData.UpdateEntries(mc.app); err != nil {
		return err
	}

	return nil
}

func (e *entry) Execute(mc *matrixConfig, roomID id.RoomID, src *data.Source) {
	de := e.ToEntry()
	if de == nil {
		return
	}

	de.Source = src

	switch e.Action {
	case "add":
		if err := mc.app.CreateEntry(de); err != nil {
			mc.app.Logger().Error("Failed to create entry", "error", err)
			return
		}
		mc.aiData.AddChatHistory("assistant", fmt.Sprintf("added task: %v", e))
		mc.sendMessage(roomID, fmt.Sprintf("Creating task: %+v", de))
	case "delete":
		id, err := mc.app.FindEntryByRemoteID(mc.sourceID, de)
		if err != nil {
			mc.app.Logger().Error("Failed to find entry", "error", err)
			return
		}

		de.ID = id

		if err := mc.app.DeleteEntry(de); err != nil {
			mc.app.Logger().Error("Failed to delete entry", "error", err)
			return
		}

		mc.aiData.AddChatHistory("assistant", fmt.Sprintf("deleted task: %v", e))
		mc.sendMessage(roomID, fmt.Sprintf("Deleting task: %+v", de))
	}
}

func (mc *matrixConfig) calculateResponse(roomID id.RoomID, sender id.UserID, input string) (string, error) {
	input = strings.TrimSpace(input)
	switch input {
	case "":
		return "", nil
	case "ping":
		return "*pong back*", nil
	}

	mc.app.Logger().Info("Parsing question...", "sender", sender)
	mc.aiData.EmployerQuestion = []string{fmt.Sprintf("Sender: %s", sender), input}

	if err := mc.aiData.UpdateEntries(mc.app); err != nil {
		return "", err
	}

	isTask, err := mc.aiClient.GeneratePrompt(context.Background(), ai.PromptCheckTask, mc.aiData.EmployerQuestion)
	if err != nil {
		return "", err
	}

	if isTask = strings.TrimSpace(isTask); isTask != "0" {
		mc.sendMessage(roomID, isTask)
		mc.app.Logger().Info("Calculating tasks...")

		err := mc.performTasks(roomID, sender, input)
		if err != nil {
			return "", fmt.Errorf("could not perform task: %w", err)
		}
	}

	md, err := mc.aiClient.GeneratePrompt(context.Background(), ai.PromptCustom, mc.aiData)
	if err != nil {
		return "", fmt.Errorf("could not calculate response: %w", err)
	}

	mc.aiData.AddChatHistory("user", input)
	mc.aiData.AddChatHistory("assistant", md)

	return md, nil
}

func (mc *matrixConfig) syncFunc(ctx context.Context, evt *event.Event) {
	if evt.GetStateKey() != mc.client.UserID.String() || evt.Content.AsMember().Membership != event.MembershipInvite {
		return
	}

	if _, err := mc.client.JoinRoomByID(ctx, evt.RoomID); err != nil {
		mc.app.Logger().Error(
			"Failed to join room after invite",
			"room_id", evt.RoomID.String(),
			"inviter", evt.Sender.String(),
			"error", err,
		)
		return
	}

	mc.app.Logger().Info(
		"Joined room after invite",
		"room_id", evt.RoomID.String(),
		"inviter", evt.Sender.String(),
	)
}

func (mc *matrixConfig) initClient() error {
	client, err := mautrix.NewClient(mc.homeserver, "", "")
	if err != nil {
		return err
	}

	mc.client = client

	return nil
}

func (mc *matrixConfig) configureCryptoHelper() error {
	cryptoHelper, err := cryptohelper.NewCryptoHelper(mc.client, []byte("meow"), mc.database)
	if err != nil {
		return err
	}

	cryptoHelper.LoginAs = &mautrix.ReqLogin{
		Type:       mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: mc.username},
		Password:   mc.password,
	}

	if err := cryptoHelper.Init(context.TODO()); err != nil {
		return err
	}

	mc.cryptoHelper = cryptoHelper
	mc.client.Crypto = cryptoHelper

	return nil
}

func (mc *matrixConfig) initChat() {
	mc.msges = make(chan msg, 10)

	go func() {
		for msg := range mc.msges {
			resp, err := mc.client.SendMessageEvent(context.Background(), msg.RoomID, event.EventMessage, msg.Content)
			if err != nil {
				mc.app.Logger().Error("Error sending event", "error", err)
				continue
			}

			mc.app.Logger().Info("Event sent", "event_id", resp.EventID.String())
		}
	}()
}

func (mc *matrixConfig) sendMessage(roomID id.RoomID, text string) {
	content := format.RenderMarkdown(text, true, true)

	mc.msges <- msg{RoomID: roomID, Content: content}
}
