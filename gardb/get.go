package gardb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
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
	data, class, err := c.apiClient.Get(ctx, id)
	if err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to get object from API: %v", class, err),
		}
	}

	// Decrypt DEK
	ptDEK, class, err := c.enclaveClient.DecryptDEK(ctx, id, base64.StdEncoding.EncodeToString(data.DEK))
	if err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to decrypt DEK: %v", class, err),
		}
	}

	// Decrypt object
	decryptedObjBytes, err := crypto.DecryptObjectProbabilistic(data.EncryptedObj, ptDEK)
	if err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to decrypt object: %v", errors.ErrEncryption, err),
		}
	}

	err = json.Unmarshal(decryptedObjBytes, obj)
	if err != nil {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: failed to unmarshal decrypted object JSON: %v", errors.ErrEncryption, err),
		}
	}

	// Add metadata to the object
	now := data.CreatedAt
	rv := reflect.ValueOf(obj).Elem()
	updatedAtField := rv.FieldByName("UpdatedAt")
	if updatedAtField.IsValid() && updatedAtField.CanSet() && updatedAtField.Type() == reflect.TypeOf(time.Time{}) {
		updatedAtField.Set(reflect.ValueOf(now))
	}
	createdAtField := rv.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
		if createdAtField.IsZero() {
			createdAtField.Set(reflect.ValueOf(now))
		}
	}
	idField := rv.FieldByName("ID")
	if idField.IsValid() && idField.CanSet() && idField.Type() == reflect.TypeOf("") {
		if idField.String() == "" {
			idField.SetString(id)
		}
	}

	return nil
}
