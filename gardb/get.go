package gardb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

func (c *Client) Get(ctx context.Context, id string, obj any) error {
	// Validate that ptrToStruct is a pointer to a struct that has a GardbMeta field
	if !internal.ValidatePtrToStructWithGardbMeta(obj) {
		return ErrInvalidObjectType
	}

	// Call the API client's Get method to retrieve the encrypted object payload
	data, err := c.apiClient.Get(ctx, id)
	if err != nil {
		return err
	}

	// Decrypt DEK
	ptDEK, err := c.enclaveClient.DecryptDEK(ctx, id, base64.StdEncoding.EncodeToString(data.DEK))
	if err != nil {
		return ErrDecryptionFailed
	}

	// Decrypt object
	decryptedObjBytes, err := crypto.DecryptObjectProbabilistic(data.EncryptedObj, ptDEK)
	if err != nil {
		return ErrDecryptionFailed
	}

	err = json.Unmarshal(decryptedObjBytes, obj)
	if err != nil {
		return ErrDecryptionFailed
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
