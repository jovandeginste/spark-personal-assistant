package mail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
