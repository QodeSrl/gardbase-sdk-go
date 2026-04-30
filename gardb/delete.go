package gardb

import (
	"context"
	"fmt"

	"github.com/qodesrl/gardbase-sdk-go/gardb/errors"
	"github.com/qodesrl/gardbase-sdk-go/internal"
)

func (s *GardbSchema[T]) Delete(ctx context.Context, id string) error {
	const op = "Schema.Delete"

	err := s.client.apiClient.Delete(ctx, s.tableHash, id)
	if err != nil {
		if internal.IsContextError(err) {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	return nil
}
