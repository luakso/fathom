package base

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// DefaultBackoff is the production retry schedule: 5s, 25s, 120s, 120s, 120s
// (5 retries, capped at 120s). Matches spec §11.
var DefaultBackoff = []time.Duration{
	5 * time.Second,
	25 * time.Second,
	120 * time.Second,
	120 * time.Second,
	120 * time.Second,
}

// WithRateLimitBackoff calls fn and, on rate-limit-shaped errors, retries up
// to len(sleeps) times with the given inter-call sleeps. Non-rate-limit errors
// fail fast.
//
// "Rate-limit-shaped" is detected via substring match — "429" or "rate limit"
// or "too many requests" (case-insensitive). This is intentionally
// permissive: silent retry on a schema/auth/decode error wastes hours, but a
// permissive 429 detector costs only one extra retry on rare false positives.
//
// Context cancellation during a sleep returns ctx.Err() immediately.
func WithRateLimitBackoff(ctx context.Context, fn func() error, sleeps []time.Duration) error {
	err := fn()
	if err == nil {
		return nil
	}
	if !isRateLimited(err) {
		return err
	}
	for attempt, d := range sleeps {
		slog.Warn(
			"rpc: rate-limited, backing off",
			"attempt", attempt+1,
			"backoff", d.String(),
			"err", err,
		)
		select {
		case <-ctx.Done():
			return fmt.Errorf("ctx cancelled during backoff: %w", ctx.Err())
		case <-time.After(d):
		}
		err = fn()
		if err == nil {
			return nil
		}
		if !isRateLimited(err) {
			return err
		}
	}
	return fmt.Errorf("rate limit retries exhausted: %w", err)
}

func isRateLimited(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests")
}
