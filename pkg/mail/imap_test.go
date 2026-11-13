package mail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMultipartMessageSanitizesHeaders(t *testing.T) {
	msg := string(buildMultipartMessage(
		"sender@example.com",
		[]string{"to@example.com"},
		"Hello\r\nBcc: evil@example.com",
		"Body",
	))

	assert.NotContains(t, msg, "\r\nBcc:")
	assert.NotContains(t, msg, "\nBcc:")
	assert.Contains(t, msg, "Subject: Hello Bcc: evil@example.com\r\n")
}

func TestBuildMultipartMessageEncodesNonASCII(t *testing.T) {
	msg := string(buildMultipartMessage(
		"sender@example.com",
		[]string{"to@example.com"},
		"Héllo",
		"Body",
	))

	assert.Contains(t, msg, "Subject: =?utf-8?q?H=C3=A9llo?=\r\n")
	assert.False(t, strings.Contains(msg, "Subject: Héllo"))
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
