package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func runWithCapture(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer
	oldOut := cliStdout
	oldErr := cliStderr
	cliStdout = &out
	cliStderr = &errOut
	t.Cleanup(func() {
		cliStdout = oldOut
		cliStderr = oldErr
	})
	err := run(context.Background(), args)
	return out.String(), errOut.String(), err
}

func TestRunNoArgsPrintsTopLevelHelp(t *testing.T) {
	out, _, err := runWithCapture(t)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out, "cheap-rag CLI") {
		t.Fatalf("expected top-level help purpose, got: %s", out)
	}
	if !strings.Contains(out, "Commands:") {
		t.Fatalf("expected command list in help, got: %s", out)
	}
	if strings.Contains(out, "chatbot") {
		t.Fatalf("did not expect legacy CLI name in help: %s", out)
	}
}

func TestRunHelpFlagsPrintTopLevelHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}} {
		out, _, err := runWithCapture(t, args...)
		if err != nil {
			t.Fatalf("expected no error for %v, got %v", args, err)
		}
		if !strings.Contains(out, "Usage:") || !strings.Contains(out, "cheaprag <command>") {
			t.Fatalf("expected top-level help usage for %v, got: %s", args, out)
		}
	}
}

func TestIndexHelpOutput(t *testing.T) {
	out, _, err := runWithCapture(t, "index", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out, "Usage:\n  cheaprag index") {
		t.Fatalf("expected cheaprag index usage, got: %s", out)
	}
	if !strings.Contains(out, "Config and flags:") {
		t.Fatalf("expected config-vs-flag guidance, got: %s", out)
	}
}

func TestInspectQueryHelpOutput(t *testing.T) {
	out, _, err := runWithCapture(t, "inspect", "query", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out, "cheaprag inspect query") {
		t.Fatalf("expected inspect query usage with cheaprag name, got: %s", out)
	}
}
