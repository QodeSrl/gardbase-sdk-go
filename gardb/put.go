package gardb

import (
	"context"
	"fmt"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
)

// Put stores an object in the GardBase database.
//
// It validates the object against the schema, extracts values and indexes,
// generates a Data Encryption Key (DEK) from the enclave, and uploads the
// encrypted data to the database. Upon success, it updates the object's
// metadata with the assigned ID and timestamps.
//
// Parameters:
//   - ctx: The context for managing request cancellation and timeout
//   - obj: The object to be stored, which must implement the GardbObject interface
//
// Returns:
//   - An error if any step of the validation, encryption, or upload process fails, or if the context is cancelled/times out
func (s *gardbSchema[T]) Put(ctx context.Context, obj T) error {
	const op = "Schema.Put"

	if err := ctx.Err(); err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("context error: %w", err),
		}
	}

	// Validate obj against the schema and initialize GardbMeta
	if err := s.validate(op, obj); err != nil {
		return err
	}

	// Extract values and indexes from the object using the schema
	values, indexes, err := s.extract(obj)
	if err != nil {
		// return error, schema.Extract already returns *errors.Error
		return err
	}

	// Generate a DEK using the enclave client
	deks, err := s.client.enclaveClient.GenerateDEK(ctx, 1)
	if err != nil {
		if internal.IsContextError(err) {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to generate DEK: %v", errors.ErrSession, err),
		}
	}
	if len(deks) == 0 {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: no DEKs returned from enclave", errors.ErrSession),
		}
	}

	// Call the API client's Put method to handle encryption and upload
	respBody, err := s.client.apiClient.Put(ctx, values, indexes, deks[0], s.name, s.tableHash)
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

	meta := obj.getGardbMeta()

	// Update ID
	meta.ID = respBody.ObjectID
	// Update timestamps
	meta.CreatedAt = respBody.CreatedAt
	meta.UpdatedAt = respBody.CreatedAt

	return nil
}
