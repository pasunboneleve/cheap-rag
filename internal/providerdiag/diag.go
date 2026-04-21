package providerdiag

import (
	"context"
	"sync"
)

type tracker struct {
	mu       sync.Mutex
	statuses map[string]int
}

type trackerKey struct{}
type stageKey struct{}

func WithTracker(parent context.Context) (context.Context, *tracker) {
	t := &tracker{statuses: map[string]int{}}
	return context.WithValue(parent, trackerKey{}, t), t
}

func WithStage(ctx context.Context, stage string) context.Context {
	return context.WithValue(ctx, stageKey{}, stage)
}

func RecordStatus(ctx context.Context, status int) {
	t, ok := ctx.Value(trackerKey{}).(*tracker)
	if !ok || t == nil {
		return
	}
	stage, ok := ctx.Value(stageKey{}).(string)
	if !ok || stage == "" {
		return
	}
	t.mu.Lock()
	t.statuses[stage] = status
	t.mu.Unlock()
}

func (t *tracker) Snapshot() map[string]int {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.statuses) == 0 {
		return nil
	}
	out := make(map[string]int, len(t.statuses))
	for k, v := range t.statuses {
		out[k] = v
	}
	return out
}
