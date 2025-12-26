package gardb

import (
	"context"
	"fmt"
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
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
func (c *Client) Put(ctx context.Context, obj any) error {
	const op = "Client.Put"

	if err := ctx.Err(); err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("context error: %w", err),
		}
	}

	// Validate that ptrToStruct is a pointer to a struct that has a GardbMeta field
	if !internal.ValidatePtrToStructWithGardbMeta(obj) {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: expected pointer to struct with GardbMeta field", errors.ErrValidation),
		}
	}

	// Get the schema from the GardbMeta field
	rv := reflect.ValueOf(obj).Elem()
	metaField := rv.FieldByName("GardbMeta")

	meta, ok := metaField.Interface().(schema.GardbMeta)
	if !ok {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: GardbMeta field has wrong type", errors.ErrValidation),
		}
	}
	schema := meta.Schema()
	if schema == nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: schema not initialized", errors.ErrValidation),
		}
	}

	// Extract values and indexes from the object using the schema
	values, indexes, err := schema.Extract(obj)
	if err != nil {
		// return error, schema.Extract already returns *errors.Error
		return err
	}

	// Generate a DEK using the enclave client
	deks, err := c.enclaveClient.GenerateDEK(ctx, 1)
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

	// Call the API client's Put method to handle encryption and upload
	respBody, err := c.apiClient.Put(ctx, values, indexes, deks[0], schema)
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

	// Update CreatedAt and UpdatedAt fields in the original object
	if metaField.IsValid() && metaField.CanSet() {
		// Update ID
		meta.ID = respBody.ObjectID
		// Update timestamps
		meta.CreatedAt = respBody.CreatedAt.Unix()
		meta.UpdatedAt = respBody.CreatedAt.Unix()

		metaField.Set(reflect.ValueOf(meta))
	}

	return nil
}
