package generic

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"100.64.0.1", true},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2606:4700:4700::1111", false},
		{"::1", true},
		{"fd00::1", true},
		{"fe80::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			assert.Equal(t, tt.blocked, isBlockedIP(net.ParseIP(tt.ip)))
		})
	}
}

func TestGetBodySafeRejectsFileScheme(t *testing.T) {
	_, err := getBodySafe("file:///etc/passwd")
	require.Error(t, err)
}

func TestGetBodySafeRejectsPrivateAddress(t *testing.T) {
	_, err := getBodySafe("http://127.0.0.1:1/")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBlockedAddress)
}
