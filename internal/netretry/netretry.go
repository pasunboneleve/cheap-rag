package netretry

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"
)

const (
	maxAttempts         = 2
	retryBaseBackoff    = 120 * time.Millisecond
	retryJitterRangeMax = 80 * time.Millisecond
)

func MaxAttempts() int {
	return maxAttempts
}

func ShouldRetryStatus(status int) bool {
	return status == 429 || status >= 500
}

func ShouldRetryTransport(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

func Backoff(attempt int, rnd *rand.Rand) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	var jitterNanos int64
	if rnd == nil {
		jitterNanos = rand.Int63n(retryJitterRangeMax.Nanoseconds() + 1)
	} else {
		jitterNanos = rnd.Int63n(retryJitterRangeMax.Nanoseconds() + 1)
	}
	return retryBaseBackoff + time.Duration(jitterNanos)
}

func SleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
