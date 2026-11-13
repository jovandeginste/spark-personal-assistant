package app

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloseRunsCleanupOnce(t *testing.T) {
	a := NewApp()

	var calls atomic.Int32

	a.mcpCleanup["test"] = func() {
		calls.Add(1)
	}

	a.Close()
	a.Close()

	assert.Equal(t, int32(1), calls.Load())
}
