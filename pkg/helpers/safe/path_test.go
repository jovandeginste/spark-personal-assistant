package safe

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSubPath(t *testing.T) {
	// Setup temporary directory structure for testing
	tempDir := t.TempDir()
	baseDir := filepath.Join(tempDir, "base")
	subDir := filepath.Join(baseDir, "subdir")
	outsideDir := filepath.Join(tempDir, "outside")

	tests := []struct {
		name        string
		base        string
		target      string
		shouldError bool
	}{
		{
			name:        "Direct child file",
			base:        baseDir,
			target:      filepath.Join(baseDir, "file.txt"),
			shouldError: false,
		},
		{
			name:        "Subdirectory child file",
			base:        baseDir,
			target:      filepath.Join(subDir, "file.txt"),
			shouldError: false,
		},
		{
			name:        "Same directory",
			base:        baseDir,
			target:      baseDir,
			shouldError: false,
		},
		{
			name:        "Path traversal attempt",
			base:        baseDir,
			target:      filepath.Join(baseDir, "../outside/file.txt"),
			shouldError: true,
		},
		{
			name:        "Explicit outside path",
			base:        baseDir,
			target:      filepath.Join(outsideDir, "file.txt"),
			shouldError: true,
		},
		{
			name:        "Root against sub",
			base:        baseDir,
			target:      tempDir, // Parent of base
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsSubPath(tt.base, tt.target)
			if tt.shouldError {
				require.Error(t, err, "Base: %s, Target: %s", tt.base, tt.target)
			} else {
				require.NoError(t, err, "Base: %s, Target: %s", tt.base, tt.target)
			}
		})
	}
}

func TestIsSubPathWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific path tests on non-Windows OS")
	}

	tests := []struct {
		name   string
		base   string
		target string
	}{
		{
			name:   "Different drives",
			base:   "C:\\Users",
			target: "D:\\Data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsSubPath(tt.base, tt.target)
			require.Error(t, err) // filepath.Rel fails across drives
		})
	}
}
