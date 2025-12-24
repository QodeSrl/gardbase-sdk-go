package gardb

import (
	"context"
	"reflect"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func (c *Client) Put(ctx context.Context, obj any) error {
	// Validate that ptrToStruct is a pointer to a struct that has a GardbMeta field
	if !internal.ValidatePtrToStructWithGardbMeta(obj) {
		return ErrInvalidObjectType
	}

	// Get the schema from the GardbMeta field
	schema := reflect.ValueOf(obj).Elem().FieldByName("GardbMeta").Interface().(*schema.GardbMeta).Schema()

	// Extract values and indexes from the object using the schema
	values, indexes, err := schema.Extract(obj)
	if err != nil {
		return err
	}

	// Generate a DEK using the enclave client
	dek, err := c.enclaveClient.GenerateDEK(ctx, 1)
	if err != nil {
		return err
	}

	// Call the API client's Put method to handle encryption and upload
	respBody, err := c.apiClient.Put(ctx, values, indexes, dek[0], schema)
	if err != nil {
		return err
	}

	// Update CreatedAt and UpdatedAt fields in the original object
	rv := reflect.ValueOf(obj).Elem()
	now := respBody.CreatedAt
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

	return nil
}
