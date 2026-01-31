package gardb

import (
	"context"
	"fmt"
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
)

// Put validates and persists obj to Gardb.
//
// Put expects obj to be a pointer to a struct that contains a GardbMeta field.
// Put obtains the schema from obj.GardbMeta, extracts values and indexes
// via schema.Extract, and requests a data-encryption key (DEK) from the enclave client. It uses the first returned DEK to
// encrypt and upload the extracted values and indexes by calling the API client's Put method.
//
// On success Put updates the object's UpdatedAt field with the timestamp returned by the API and sets CreatedAt to the same
// timestamp only if CreatedAt was previously the zero value. Context cancellation or timeouts are propagated to DEK
// generation and the API call. Any error from validation, extraction, DEK generation, or the API upload is returned.
//
// Side effects: mutates the provided obj (CreatedAt/UpdatedAt) and performs network/enclave operations.
func (s *Schema) Put(ctx context.Context, obj any) error {
	const op = "Schema.Put"

	if err := ctx.Err(); err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("context error: %w", err),
		}
	}

	// Validate that obj is a pointer to a struct
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: expected pointer to struct, got %T", errors.ErrInvalidSchema, obj),
		}
	}

	// Validate obj against the schema and initialize GardbMeta
	if err := s.new(op, obj); err != nil {
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

	metaField := reflect.ValueOf(obj).Elem().FieldByName("GardbMeta")
	// Update CreatedAt and UpdatedAt fields in the original object
	if metaField.IsValid() && metaField.CanSet() {
		meta := metaField.Interface().(GardbMeta)
		// Update ID
		meta.ID = respBody.ObjectID
		// Update timestamps
		meta.CreatedAt = respBody.CreatedAt
		meta.UpdatedAt = respBody.CreatedAt

		metaField.Set(reflect.ValueOf(meta))
	}

	return nil
}
