package fsguard

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Guard enforces explicit runtime/content boundaries.
type Guard struct {
	contentRoot string
	runtimeRoot string
}

func New(contentRoot, runtimeRoot string) (*Guard, error) {
	if contentRoot == "" || runtimeRoot == "" {
		return nil, errors.New("contentRoot and runtimeRoot are required")
	}
	contentAbs, err := filepath.Abs(contentRoot)
	if err != nil {
		return nil, fmt.Errorf("content root abs: %w", err)
	}
	runtimeAbs, err := filepath.Abs(runtimeRoot)
	if err != nil {
		return nil, fmt.Errorf("runtime root abs: %w", err)
	}
	contentResolved, err := resolveExisting(contentAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve content root: %w", err)
	}
	runtimeResolved, err := resolveForCreate(runtimeAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve runtime root: %w", err)
	}
	if within(runtimeResolved, contentResolved) {
		return nil, errors.New("runtime root must not be inside content root")
	}
	if within(contentResolved, runtimeResolved) {
		return nil, errors.New("content root must not be inside runtime root")
	}
	return &Guard{contentRoot: contentResolved, runtimeRoot: runtimeResolved}, nil
}

func (g *Guard) ContentRoot() string { return g.contentRoot }
func (g *Guard) RuntimeRoot() string { return g.runtimeRoot }

func (g *Guard) ResolveContentFile(path string) (string, error) {
	return g.resolveUnder(path, g.contentRoot)
}

func (g *Guard) ResolveRuntimeFile(path string) (string, error) {
	return g.resolveUnder(path, g.runtimeRoot)
}

func (g *Guard) EnsureRuntimeDir() error {
	if err := os.MkdirAll(g.runtimeRoot, 0o755); err != nil {
		return fmt.Errorf("create runtime root: %w", err)
	}
	return nil
}

func (g *Guard) WalkContent(fn fs.WalkDirFunc) error {
	return filepath.WalkDir(g.contentRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path != g.contentRoot {
			if _, err := g.resolveUnder(path, g.contentRoot); err != nil {
				return err
			}
		}
		return fn(path, d, nil)
	})
}

func (g *Guard) resolveUnder(path, root string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, path)
	}
	clean := filepath.Clean(candidate)
	resolved, err := resolveForCreate(clean)
	if err != nil {
		return "", err
	}
	if !within(resolved, root) {
		return "", fmt.Errorf("path escapes root: %s", path)
	}
	return resolved, nil
}

func resolveExisting(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func resolveForCreate(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		return filepath.Clean(resolved), nil
	}
	parent := filepath.Dir(path)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(resolvedParent, filepath.Base(path))), nil
}

func within(path, root string) bool {
	p := filepath.Clean(path)
	r := filepath.Clean(root)
	if p == r {
		return true
	}
	if !strings.HasSuffix(r, string(filepath.Separator)) {
		r += string(filepath.Separator)
	}
	return strings.HasPrefix(p, r)
}
