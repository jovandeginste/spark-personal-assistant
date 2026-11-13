package safe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// IsSubPath verifies if a target path is inside a base path.
// It returns nil if target is a subdirectory of or equal to base,
// otherwise it returns an error. Symlinks are resolved before the check.
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

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target path %s is not within base path %s", target, base)
	}

	resolved, err := securejoin.SecureJoin(absBase, rel)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}

	resolvedTarget, err := filepath.EvalSymlinks(absTarget)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to resolve target path: %w", err)
		}

		resolvedTarget = absTarget
	}

	if filepath.Clean(resolved) != filepath.Clean(resolvedTarget) {
		return fmt.Errorf("target path %s is not within base path %s", target, base)
	}

	return nil
}
