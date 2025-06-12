package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/md"
	"github.com/orcaman/writerseeker"
	"github.com/spf13/cobra"
)

func (c *cli) md2speechCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "md2speech file.md file.Wav",
		Short:   "Convert markdown to audio",
		Example: "spark md2speech ./md/summary-full.md ./output.wav",
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

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant, c.app.Logger())
			if err != nil {
				return err
			}

			wavBytes, err := aiClient.GenerateSpeech(context.Background(), text)
			if err != nil {
				return err
			}

			c.app.Logger().Info("Sending audio...")

			w := writerseeker.WriterSeeker{}

			if err := pcmToWav(bytes.NewReader(wavBytes), &w); err != nil {
				return err
			}

			out := os.Stdout

			if args[1] != "-" {
				out, err = os.Create(args[1])
				if err != nil {
					return err
				}

				defer out.Close()
			}

			_, err = io.Copy(out, w.Reader())
			return err
		},
	}

	return cmd
}

func pcmToWav(in io.Reader, out io.WriteSeeker) error {
	// 24 kHz, 16 bit, 1 channel, WAV.
	e := wav.NewEncoder(out, 24000, 16, 1, 1)

	// Create new audio.IntBuffer.
	audioBuf, err := newAudioIntBuffer(in)
	if err != nil {
		return err
	}

	defer e.Close()

	// Write buffer to output file. This writes a RIFF header and the PCM chunks from the audio.IntBuffer.
	return e.Write(audioBuf)
}

func newAudioIntBuffer(r io.Reader) (*audio.IntBuffer, error) {
	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  24000,
		},
	}

	for {
		var sample int16

		err := binary.Read(r, binary.LittleEndian, &sample)

		switch {
		case err == io.EOF:
			return &buf, nil
		case err != nil:
			return nil, err
		}

		buf.Data = append(buf.Data, int(sample))
	}
}
