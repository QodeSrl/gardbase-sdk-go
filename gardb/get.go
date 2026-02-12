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

// Get retrieves a decrypted object from the database by its ID.
//
// It fetches the encrypted object payload from the API client, decrypts the Data Encryption Key (DEK)
// through the enclave client, decrypts the object using the plaintext DEK, and unmarshals the result
// into the specified type T. The object's metadata (ID, CreatedAt, UpdatedAt) is populated from the
// retrieved data.
//
// Parameters:
//   - ctx: The context for managing request cancellation and timeout
//   - id: The unique identifier of the object to retrieve
//
// Returns:
//   - An object of type T containing the decrypted and unmarshalled data
//   - An error if any step of the retrieval or decryption process fails, or if the context is cancelled/times out
func (s *gardbSchema[T]) Get(ctx context.Context, id string) (T, error) {
	const op = "Schema.Get"

	var obj T

	// Call the API client's Get method to retrieve the encrypted object payload
	data, err := s.client.apiClient.Get(ctx, s.tableHash, id)
	if err != nil {
		if internal.IsContextError(err) {
			return obj, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return obj, &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	// Decrypt DEK
	ptDEK, err := s.client.enclaveClient.DecryptDEKs(ctx, []internal.DecryptDEKObject{
		{
			ObjectID: id,
			DEKB64:   base64.StdEncoding.EncodeToString(data.KMSWrappedDEK),
		},
	})
	if err != nil {
		if internal.IsContextError(err) {
			return obj, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return obj, &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	// Decrypt object
	decryptedObjBytes, err := crypto.DecryptObjectProbabilistic(data.EncryptedObj, ptDEK[0].DEK)
	if err != nil {
		if internal.IsContextError(err) {
			return obj, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return obj, &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to decrypt object: %v", errors.ErrEncryption, err),
		}
	}

	var raw map[string]any
	if err = json.Unmarshal(decryptedObjBytes, &raw); err != nil {
		return obj, &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to unmarshal object: %v", errors.ErrEncryption, err),
		}
	}
	obj = reflect.New(reflect.TypeOf(obj).Elem()).Interface().(T)
	if err = s.populate(obj, raw); err != nil {
		return obj, &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	meta := obj.getGardbMeta()

	meta.ID = id
	meta.CreatedAt = data.CreatedAt
	meta.UpdatedAt = data.UpdatedAt

	return obj, nil
}
