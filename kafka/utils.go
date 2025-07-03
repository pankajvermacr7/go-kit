package kafka

import (
	"context"
	"time"
)

func RetryOperation(ctx context.Context, maxRetries int, initialBackoff time.Duration, fn func() error) error {
	var err error
	backoff := initialBackoff

	for i := 0; i < maxRetries; i++ {
		if err = fn(); err == nil {
			return nil
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
