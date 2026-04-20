package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dmvianna/cheap-rag/internal/types"
)

type fakeAsker struct {
	out types.AskOutcome
	err error
}

func (f fakeAsker) Ask(context.Context, string) (types.AskOutcome, error) {
	return f.out, f.err
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	srv := New(fakeAsker{}, "", discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAskRequiresAuthWhenTokenConfigured(t *testing.T) {
	t.Parallel()
	srv := New(fakeAsker{}, "secret", discardLogger())
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"question":"hello"}`))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAskReturnsJSON(t *testing.T) {
	t.Parallel()
	asker := fakeAsker{
		out: types.AskOutcome{
			Answer: "ok",
			Retrieved: []types.RetrievalResult{{
				Chunk:      types.Chunk{ID: "c1", Citation: "slug", Path: "a.md"},
				Similarity: 0.7,
			}},
		},
	}
	srv := New(asker, "", discardLogger())
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"question":"hello"}`))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["answer"] != "ok" {
		t.Fatalf("expected answer 'ok', got %#v", body["answer"])
	}
}

func TestListenUnixSocketCleansUp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "cheap-rag.sock")
	ln, cleanup, err := ListenUnixSocket(socketPath)
	if err != nil {
		t.Fatalf("listen socket: %v", err)
	}
	if ln == nil || cleanup == nil {
		t.Fatalf("expected listener and cleanup")
	}
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("socket should exist: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket should be removed, stat err=%v", err)
	}
}

func discardLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
