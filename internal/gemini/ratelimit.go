package gemini

import (
	"context"
	"time"
)

type RateLimiter struct {
	ticker *time.Ticker
}

func NewRateLimiter(rpm int) *RateLimiter {
	if rpm < 1 {
		rpm = 1
	}

	interval := time.Minute / time.Duration(rpm)
	return &RateLimiter{ticker: time.NewTicker(interval)}
}

// Wait blocks until the next tick, or returns ctx.Err() if cancelled first.
func (r *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-r.ticker.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *RateLimiter) Stop() { r.ticker.Stop() }
