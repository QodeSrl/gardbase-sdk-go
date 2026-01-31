package gardb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

// Get retrieves the object identified by id from Gardb and unmarshals it into obj.
//
// Get expects obj to be a pointer to a struct that contains a GardbMeta field.
// Get retrieves the encrypted object with the given id from the remote API, decrypts its data encryption key (DEK)
// using the enclave client, decrypts the stored object payload, and unmarshals the resulting JSON into obj.
//
// On success Get updates the GardbMeta fields in obj with the ID, CreatedAt, and UpdatedAt values returned by the server.
//
// Parameters:
//   - ctx: context for API and enclave operations.
//   - id: identifier of the object to fetch.
//   - obj: destination object (pointer to struct) to unmarshal the decrypted JSON into.
//
// Returns an error if the provided obj is invalid (ErrInvalidObjectType), if the API call fails, or if DEK/object decryption
// or JSON unmarshalling fails (returns ErrDecryptionFailed for decryption/unmarshal failures).
func (s *Schema) Get(ctx context.Context, id string, obj any) error {
	const op = "Schema.Get"

	// Validate that ptrToStruct is a pointer to a struct that has a GardbMeta field
	if !validatePtrToStructWithGardbMeta(obj) {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: expected pointer to struct with GardbMeta field", errors.ErrValidation),
		}
	}

	// Call the API client's Get method to retrieve the encrypted object payload
	data, err := s.client.apiClient.Get(ctx, s.tableHash, id)
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
	ptDEK, err := s.client.enclaveClient.DecryptDEK(ctx, id, base64.StdEncoding.EncodeToString(data.KMSWrappedDEK))
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

	meta, ok := metaField.Interface().(GardbMeta)
	if !ok {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: invalid GardbMeta type", errors.ErrValidation),
		}
	}
	meta.schema = s
	meta.ID = id
	meta.CreatedAt = data.CreatedAt
	meta.UpdatedAt = data.UpdatedAt

	metaField.Set(reflect.ValueOf(meta))

	return nil
}
