package gardb

import (
	"context"
	"errors"
	"time"
)

type UpdateOptions struct {
	MaxRetries int
	RetryDelay time.Duration
}

func defaultUpdateOption() UpdateOptions {
	return UpdateOptions{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}
}

type UpdateOption func(*UpdateOptions)

func WithMaxRetries(n int) UpdateOption {
	return func(o *UpdateOptions) {
		o.MaxRetries = n
	}
}

func WithRetryDelay(d time.Duration) UpdateOption {
	return func(o *UpdateOptions) {
		o.RetryDelay = d
	}
}

func (s *gardbSchema[T]) Update(ctx context.Context, id string, mutateFn func(dest T) error, opts ...UpdateOption) error {
	return errors.New("not implemented yet")
}
