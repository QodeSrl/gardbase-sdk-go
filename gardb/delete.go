package gardb

import "context"

func (s *gardbSchema[T]) Delete(ctx context.Context, id string) error {
	const op = "Schema.Delete"

	return nil
}
