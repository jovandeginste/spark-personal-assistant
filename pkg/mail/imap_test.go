package mail

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLiteral struct {
	*bytes.Reader
}

func (l testLiteral) Size() int64 {
	return int64(l.Len())
}

func TestConnectAndIdleFetchesUnseen(t *testing.T) {
	memServer := imapmemserver.New()

	user := imapmemserver.NewUser("user", "pass")
	require.NoError(t, user.Create("INBOX", nil))

	rawMessage := "From: sender@example.com\r\n" +
		"To: user@example.com\r\n" +
		"Subject: Test\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Hello"

	_, err := user.Append("INBOX", testLiteral{bytes.NewReader([]byte(rawMessage))}, &imap.AppendOptions{})
	require.NoError(t, err)

	memServer.AddUser(user)

	server := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapIdle: {}},
		InsecureAuth: true,
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		_ = listener.Close()
		_ = server.Close()
	})

	host, portString, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	port, err := strconv.Atoi(portString)
	require.NoError(t, err)

	cfg := Config{
		IMAP:     IMAPConfig{Host: host, Port: port},
		Username: "user",
		Password: "pass",
		Folder:   "INBOX",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	msgChan := make(chan Message, 1)
	errChan := make(chan error, 1)

	go func() {
		errChan <- ConnectAndIdle(ctx, cfg, msgChan, logger)
	}()

	select {
	case msg := <-msgChan:
		assert.Equal(t, "Test", msg.Subject)
		assert.Equal(t, "sender@example.com", msg.FromAddress)
		assert.Contains(t, msg.Body, "Hello")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	cancel()

	select {
	case err := <-errChan:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("ConnectAndIdle did not stop")
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	assert.Equal(t, "Hello Bcc: evil@example.com", sanitizeHeaderValue("Hello\r\nBcc: evil@example.com"))
	assert.Equal(t, "Hello World", sanitizeHeaderValue("Hello\rWorld"))
	assert.Equal(t, "Hello World", sanitizeHeaderValue(" Hello\nWorld "))
}

func TestExtractMessageBody(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{
			name:     "quoted-printable",
			raw:      "Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nHello=2C World=\r\n!",
			expected: "Hello, World!",
		},
		{
			name:     "base64",
			raw:      "Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\nSGVsbG8sIFdvcmxkIQ==",
			expected: "Hello, World!",
		},
		{
			name: "multipart prefers plain text",
			raw: "Content-Type: multipart/alternative; boundary=\"b\"\r\n\r\n" +
				"--b\r\nContent-Type: text/plain\r\n\r\nplain body\r\n" +
				"--b\r\nContent-Type: text/html\r\n\r\n<b>html body</b>\r\n" +
				"--b--\r\n",
			expected: "plain body",
		},
		{
			name:     "html fallback",
			raw:      "Content-Type: text/html\r\n\r\n<p>html body</p>",
			expected: "<p>html body</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := extractMessageBody(strings.NewReader(tt.raw))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, strings.TrimSpace(body))
		})
	}
}

func TestIsAllowedSender(t *testing.T) {
	assert.True(t, isAllowedSender(Config{}, "anyone@example.com"))

	cfg := Config{Allowlist: []string{"Allowed@Example.com"}}

	assert.True(t, isAllowedSender(cfg, "allowed@example.com"))
	assert.False(t, isAllowedSender(cfg, "other@example.com"))
	assert.False(t, isAllowedSender(cfg, ""))
}
