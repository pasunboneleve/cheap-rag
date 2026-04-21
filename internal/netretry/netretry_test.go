package netretry

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestShouldRetryStatus(t *testing.T) {
	t.Parallel()
	if !ShouldRetryStatus(429) {
		t.Fatal("expected 429 to be retryable")
	}
	if !ShouldRetryStatus(500) {
		t.Fatal("expected 500 to be retryable")
	}
	if ShouldRetryStatus(401) {
		t.Fatal("expected 401 to be non-retryable")
	}
	if ShouldRetryStatus(400) {
		t.Fatal("expected 400 to be non-retryable")
	}
}

func TestShouldRetryTransport(t *testing.T) {
	t.Parallel()
	if !ShouldRetryTransport(timeoutErr{}) {
		t.Fatal("expected timeout network errors to be retryable")
	}
	if ShouldRetryTransport(errors.New("boom")) {
		t.Fatal("expected generic errors to be non-retryable")
	}
	if ShouldRetryTransport(context.Canceled) {
		t.Fatal("expected context.Canceled to be non-retryable")
	}
	var netErr net.Error = timeoutErr{}
	if !ShouldRetryTransport(netErr) {
		t.Fatal("expected net.Error timeout to be retryable")
	}
}

func TestSleepWithContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SleepWithContext(ctx, 50*time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
