package httpserver

import (
	"context"
	"encoding/json"
	"errors"
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
			ProviderStatuses: map[string]int{
				"embedding":  200,
				"generation": 200,
			},
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
	if body["outcome"] != "answer" {
		t.Fatalf("expected outcome answer, got %#v", body["outcome"])
	}
	if body["content"] != "ok" {
		t.Fatalf("expected content 'ok', got %#v", body["content"])
	}
	if body["reason"] != nil {
		t.Fatalf("expected null reason on answer, got %#v", body["reason"])
	}
	statuses, ok := body["provider_statuses"].(map[string]any)
	if !ok {
		t.Fatalf("expected provider_statuses map, got %#v", body["provider_statuses"])
	}
	if statuses["embedding"] != float64(200) || statuses["generation"] != float64(200) {
		t.Fatalf("unexpected provider statuses: %#v", statuses)
	}
}

func TestAskRefusalReturnsRefusalOutcome(t *testing.T) {
	t.Parallel()
	asker := fakeAsker{
		out: types.AskOutcome{
			Refused:       true,
			RefusalReason: "Sorry, I don't know how to answer this.",
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
	if body["outcome"] != "refusal" {
		t.Fatalf("expected outcome refusal, got %#v", body["outcome"])
	}
	if body["reason"] != "out-of-scope" {
		t.Fatalf("expected reason out-of-scope, got %#v", body["reason"])
	}
}

func TestAskErrorReturnsProviderTimeoutReason(t *testing.T) {
	t.Parallel()
	asker := fakeAsker{err: errors.New("api status 504: upstream timeout")}
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
	if body["outcome"] != "refusal" {
		t.Fatalf("expected refusal, got %#v", body["outcome"])
	}
	if body["reason"] != "provider-timeout" {
		t.Fatalf("expected provider-timeout, got %#v", body["reason"])
	}
	statuses, ok := body["provider_statuses"].(map[string]any)
	if !ok {
		t.Fatalf("expected provider_statuses map, got %#v", body["provider_statuses"])
	}
	if statuses["generation"] != float64(504) {
		t.Fatalf("expected generation status 504, got %#v", statuses["generation"])
	}
}

func TestAskErrorReturnsProviderErrorReason(t *testing.T) {
	t.Parallel()
	asker := fakeAsker{err: errors.New("db open failed")}
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
	if body["reason"] != "provider-error" {
		t.Fatalf("expected provider-error, got %#v", body["reason"])
	}
	if _, ok := body["provider_statuses"]; ok {
		t.Fatalf("expected no provider_statuses for generic error, got %#v", body["provider_statuses"])
	}
}

func TestAskErrorSetsEmbeddingProviderStatus(t *testing.T) {
	t.Parallel()
	asker := fakeAsker{err: errors.New("embed query: api status 401: unauthorized")}
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
	statuses, ok := body["provider_statuses"].(map[string]any)
	if !ok {
		t.Fatalf("expected provider_statuses map, got %#v", body["provider_statuses"])
	}
	if statuses["embedding"] != float64(401) {
		t.Fatalf("expected embedding status 401, got %#v", statuses["embedding"])
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
