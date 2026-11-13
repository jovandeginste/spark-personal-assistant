package safe

import (
	"fmt"
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

	resolvedBase := resolvePath(absBase)
	resolvedTarget := resolvePath(absTarget)

	rel, err := filepath.Rel(resolvedBase, resolvedTarget)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target path %s is not within base path %s", target, base)
	}

	resolved, err := securejoin.SecureJoin(resolvedBase, rel)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}

	if filepath.Clean(resolved) != filepath.Clean(resolvedTarget) {
		return fmt.Errorf("target path %s is not within base path %s", target, base)
	}

	return nil
}

// resolvePath resolves symlinks for the existing part of path and appends the
// non-existing remainder unchanged. This allows checking paths for files that
// have not been created yet.
func resolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}

	dir := filepath.Dir(path)
	if dir == path {
		return path
	}

	return filepath.Join(resolvePath(dir), filepath.Base(path))
}
