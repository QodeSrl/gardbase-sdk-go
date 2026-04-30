package gardb

import (
	"context"
	"time"

	stdErrors "errors"

	"github.com/qodesrl/gardbase-sdk-go/gardb/errors"
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

func (s *GardbSchema[T]) Update(ctx context.Context, id string, mutateFn func(dest T) error, opts ...UpdateOption) (T, error) {
	const op = "Schema.Update"

	options := defaultUpdateOption()
	for _, opt := range opts {
		opt(&options)
	}

	var obj T

	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		retrievedObj, err := s.Get(ctx, id)
		if err != nil {
			return obj, &errors.Error{
				Op:  op,
				Err: err,
			}
		}
		obj = retrievedObj
		if err := mutateFn(obj); err != nil {
			return obj, &errors.Error{
				Op:  op,
				Err: err,
			}
		}
		if err := s.validate(op, obj); err != nil {
			return obj, &errors.Error{
				Op:  op,
				Err: err,
			}
		}

		if err = s.Put(ctx, obj); err == nil {
			return obj, nil
		}

		if !stdErrors.Is(err, errors.ErrVersionConflict) && attempt < options.MaxRetries {
			backoff := time.Duration(attempt+1) * options.RetryDelay
			select {
			case <-ctx.Done():
				return obj, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		return obj, &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	return obj, &errors.Error{
		Op:  op,
		Err: errors.ErrMaxRetriesExceeded,
	}
}
