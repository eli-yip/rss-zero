package weibochat

import (
	"context"
	"math/rand/v2"
	"time"
)

// All clients in this process share the message API budget, across groups and runs.
var defaultMessageLimiter = newMessageLimiter()

type messageLimiter struct {
	gate        chan struct{}
	nextAllowed time.Time // Protected by gate, held until the request finishes.
	delay       func() time.Duration
}

func newMessageLimiter() *messageLimiter {
	return &messageLimiter{
		gate: make(chan struct{}, 1),
		delay: func() time.Duration {
			return time.Duration(30+rand.IntN(6)) * time.Second
		},
	}
}

// acquire serializes message requests and waits outside the HTTP timeout.
// The caller must release the gate even if it cannot authorize the request.
func (l *messageLimiter) acquire(ctx context.Context) error {
	select {
	case l.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	if remaining := time.Until(l.nextAllowed); remaining > 0 {
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			l.release()
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		l.release()
		return err
	}
	return nil
}

// started consumes an interval only when an HTTP attempt is about to start.
func (l *messageLimiter) started() {
	l.nextAllowed = time.Now().Add(l.delay())
}

func (l *messageLimiter) release() { <-l.gate }
