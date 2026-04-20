package fsguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRejectsRuntimeInsideContentRoot(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	content := filepath.Join(base, "content")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(content, filepath.Join(content, ".chatbot")); err == nil {
		t.Fatalf("expected error when runtime is inside content root")
	}
}

func TestRejectsRuntimePathTraversal(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	content := filepath.Join(base, "content")
	runtime := filepath.Join(base, "runtime")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := New(content, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.ResolveRuntimeFile("../content/leak.db"); err == nil {
		t.Fatalf("expected path traversal rejection")
	}
}

func TestRejectsContentSymlinkEscape(t *testing.T) {
	t.Parallel()
	if os.Getenv("CI") != "" {
		// Keep deterministic in environments where symlink creation may be restricted.
	}
	base := t.TempDir()
	content := filepath.Join(base, "content")
	runtime := filepath.Join(base, "runtime")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(content, "link")); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}
	g, err := New(content, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.ResolveContentFile("link/secrets.txt"); err == nil {
		t.Fatalf("expected symlink escape rejection")
	}
}
