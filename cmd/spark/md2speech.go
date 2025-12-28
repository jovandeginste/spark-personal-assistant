package main

import (
	"bytes"
	"context"
	"io"
	"os"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/md"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
)

func (c *cli) md2speechCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "md2speech file.md file.mp3",
		Short:   "Convert markdown to audio",
		Example: "spark md2speech ./md/summary-full.md ./output.mp3",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := generic.ReadResource(args[0])
			if err != nil {
				return err
			}

			text, err := md.MDToText(f)
			if err != nil {
				return err
			}

			c.app.Logger().Info("Generating speech...")

			ctx := context.Background()

			ttsc, err := texttospeech.NewClient(ctx, option.WithAPIKey(c.app.Config.TTS.APIKey))
			if err != nil {
				panic(err)
			}
			defer ttsc.Close()

			req := &texttospeechpb.SynthesizeSpeechRequest{
				Voice: &texttospeechpb.VoiceSelectionParams{
					Name:         c.app.Config.TTS.Lang + "-Chirp3-HD-" + c.app.Config.TTS.Voice,
					LanguageCode: c.app.Config.TTS.Lang,
				},
				Input: &texttospeechpb.SynthesisInput{
					InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
				},
				AudioConfig: &texttospeechpb.AudioConfig{
					AudioEncoding: texttospeechpb.AudioEncoding_MP3,
				},
			}

			resp, err := ttsc.SynthesizeSpeech(ctx, req)
			if err != nil {
				panic(err)
			}

			c.app.Logger().Info("Sending audio...")

			w := bytes.NewReader(resp.AudioContent)

			out, err := getOutput(args[1])
			defer out.Close()

			_, err = io.Copy(out, w)
			return err
		},
	}

	return cmd
}

func getOutput(dev string) (*os.File, error) {
	if dev == "-" {
		return os.Stdout, nil
	}

	return os.Create(dev)
}
