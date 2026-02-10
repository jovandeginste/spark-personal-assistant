package safe

import (
	"fmt"
	"path/filepath"
	"strings"
)

// IsSubPath verifies if a target path is inside a base path.
// It returns nil if target is a subdirectory of or equal to base,
// otherwise it returns an error.
func IsSubPath(base, target string) error {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for base: %w", err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// If relative path starts with ".." it's outside
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("target path %s is not within base path %s", target, base)
	}

	return nil
}
