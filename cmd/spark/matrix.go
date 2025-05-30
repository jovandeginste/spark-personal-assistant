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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/yarlson/pin"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type matricConfig struct {
	homeserver string
	username   string
	password   string
	database   string

	ef           app.EntryFilter
	aiData       *AIData
	aiClient     ai.Client
	client       *mautrix.Client
	app          *app.App
	cryptoHelper *cryptohelper.CryptoHelper
}

func (c *cli) matrixChatCmd() *cobra.Command {
	mc := matricConfig{app: c.app}

	cmd := &cobra.Command{
		Use:     "matrix",
		Short:   "Start Matrix client with Spark",
		Example: "spark matrix",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			aiData, err := c.buildData(mc.ef)
			if err != nil {
				return err
			}

			mc.aiData = aiData

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant)
			if err != nil {
				return err
			}

			mc.aiClient = aiClient

			if err := mc.initClient(); err != nil {
				return err
			}

			mc.configureSyncer()

			if err := mc.configureCryptoHelper(); err != nil {
				return err
			}

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
	cmd.Flags().UintVarP(&mc.ef.DaysBack, "days-back", "b", 30, "Number of days in the past to include")
	cmd.Flags().UintVarP(&mc.ef.DaysAhead, "days-ahead", "a", 90, "Number of days in the future to include")

	return cmd
}

func (mc *matricConfig) configureSyncer() {
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

		if evt.Sender.String() == mc.client.UserID.String() {
			return
		}

		if err := mc.sendResponse(evt.RoomID, evt.Sender, body); err != nil {
			mc.app.Logger().Error("Failed to send response", "error", err)
		}
	})

	syncer.OnEventType(event.StateMember, mc.syncFunc)
}

func (mc *matricConfig) sendResponse(roomID id.RoomID, sender id.UserID, input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	mc.aiData.EmployerQuestion = []string{fmt.Sprintf("Sender: %s", sender), input}

	mc.app.Logger().Info("Parsing your question...")
	spinner := pin.New("Thinking...",
		pin.WithSpinnerColor(pin.ColorCyan),
		pin.WithTextColor(pin.ColorYellow),
		pin.WithWriter(os.Stderr),
	)
	cancel := spinner.Start(context.Background())
	defer cancel()

	md, err := mc.aiClient.GeneratePrompt(context.Background(), ai.PromptCustom, mc.aiData)
	if err != nil {
		return err
	}

	spinner.Stop("Ready!")

	resp, err := mc.client.SendText(context.Background(), roomID, md)
	if err != nil {
		return err
	}

	mc.app.Logger().Info("Event sent", "event_id", resp.EventID.String())

	mc.aiData.ChatHistory = append(
		mc.aiData.ChatHistory,
		ChatHistory{Role: "user", Content: input},
		ChatHistory{Role: "assistant", Content: md},
	)

	return nil
}

func (mc *matricConfig) syncFunc(ctx context.Context, evt *event.Event) {
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

func (mc *matricConfig) initClient() error {
	client, err := mautrix.NewClient(mc.homeserver, "", "")
	if err != nil {
		return err
	}

	mc.client = client

	return nil
}

func (mc *matricConfig) configureCryptoHelper() error {
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
