package gardb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

// Get retrieves the encrypted object with the given id from the remote API, decrypts its data encryption key (DEK)
// using the enclave client, decrypts the stored object payload, and unmarshals the resulting JSON into obj.
// obj must be a pointer to a struct that contains a GardbMeta field.
// On success Get will also populate or update common metadata fields on the target struct:
//   - UpdatedAt is set to the server's CreatedAt timestamp (overwritten if present).
//   - CreatedAt is set to the server's CreatedAt timestamp only if it is currently zero.
//   - ID is set to the provided id only if the struct's ID field is an empty string.
//
// Parameters:
//   - ctx: context for API and enclave operations.
//   - id: identifier of the object to fetch.
//   - obj: destination object (pointer to struct) to unmarshal the decrypted JSON into.
//
// Returns an error if the provided obj is invalid (ErrInvalidObjectType), if the API call fails, or if DEK/object decryption
// or JSON unmarshalling fails (returns ErrDecryptionFailed for decryption/unmarshal failures).
func (c *Client) Get(ctx context.Context, id string, obj any) error {
	const op = "Client.Get"

	// Validate that ptrToStruct is a pointer to a struct that has a GardbMeta field
	if !internal.ValidatePtrToStructWithGardbMeta(obj) {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: expected pointer to struct with GardbMeta field", errors.ErrValidation),
		}
	}

	// Call the API client's Get method to retrieve the encrypted object payload
	data, err := c.apiClient.Get(ctx, id)
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

	// Decrypt DEK
	ptDEK, err := c.enclaveClient.DecryptDEK(ctx, id, base64.StdEncoding.EncodeToString(data.DEK))
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

	// Decrypt object
	decryptedObjBytes, err := crypto.DecryptObjectProbabilistic(data.EncryptedObj, ptDEK)
	if err != nil {
		if internal.IsContextError(err) {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to decrypt object: %v", errors.ErrEncryption, err),
		}
	}

	if err = json.Unmarshal(decryptedObjBytes, obj); err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to unmarshal object: %v", errors.ErrEncryption, err),
		}
	}

	// Update GardbMeta fields
	rv := reflect.ValueOf(obj).Elem()
	metaField := rv.FieldByName("GardbMeta")
	if !metaField.IsValid() {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: GardbMeta field not found", errors.ErrValidation),
		}
	}

	meta, ok := metaField.Interface().(schema.GardbMeta)
	if !ok {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: invalid GardbMeta type", errors.ErrValidation),
		}
	}
	meta.SchemaName = data.SchemaName
	meta.ID = id
	meta.CreatedAt = data.CreatedAt
	meta.UpdatedAt = data.UpdatedAt

	metaField.Set(reflect.ValueOf(meta))

	return nil
}
