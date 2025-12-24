package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
	"github.com/QodeSrl/gardbase/pkg/crypto"
	"github.com/QodeSrl/gardbase/pkg/models"
)

type APIClient struct {
	APIEndpoint string

	httpClient *http.Client
}

func NewAPIClient(apiEndpoint string, httpTimeout time.Duration) *APIClient {
	return &APIClient{
		APIEndpoint: apiEndpoint,
		httpClient:  &http.Client{Timeout: httpTimeout},
	}
}

// 1. Check if obj is a pointer to a struct
// 2. Check if gardbmeta field exists and is of type *schema.GardbMeta
// 3. Create map[string]any to hold field values and extract indexes from schema
// 4. Encrypt the object with the DEK
// 5. Encrypt each index value (TODO: with what key?)
// 6. Send the encrypted DEK and encrypted indexes to the API
// 7. Handle the API response
// 8. Upload the encrypted object to S3 using the provided upload URL
// 9. Update CreatedAt and UpdatedAt fields in the original object
func (c *APIClient) Put(ctx context.Context, obj any, dek crypto.GeneratedDEK) error {
	rv := reflect.ValueOf(obj)
	// Validate that ptrToStruct is a pointer to a struct that has a GardbMeta field
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("expected pointer to struct")
	}

	rv = rv.Elem()
	if rv.FieldByName("GardbMeta").IsValid() == false {
		return fmt.Errorf("obj must have a GardbMeta field")
	}
	if rv.FieldByName("GardbMeta").Type() != reflect.TypeOf((*schema.GardbMeta)(nil)) {
		return fmt.Errorf("GardbMeta field must be of type *schema.GardbMeta")
	}
	schema := rv.FieldByName("GardbMeta").Interface().(*schema.GardbMeta).Schema()

	values, indexes, err := schema.Extract(obj)
	if err != nil {
		return err
	}

	objBytes, err := json.Marshal(values)
	if err != nil {
		return err
	}
	encryptedObj, err := crypto.EncryptObjectProbabilistic(objBytes, dek.PlaintextDEK)
	if err != nil {
		return err
	}

	encryptedIndexesB64 := make(map[string]string, len(indexes))
	for k, v := range indexes {
		indexBytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		context := fmt.Sprintf("%s:%s", schema.Name(), k)
		encryptedIndex, err := crypto.EncryptObjectDeterministic(indexBytes, context, dek.PlaintextDEK)
		if err != nil {
			return err
		}
		encryptedIndexesB64[k] = base64.StdEncoding.EncodeToString(encryptedIndex)
	}

	body := models.CreateObjectRequest{
		EncryptedDEK: base64.StdEncoding.EncodeToString(dek.EncryptedDEK),
		Indexes:      encryptedIndexesB64,
		Sensitivity:  "medium",
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	// body should be reqBody
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody := models.CreateObjectResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return err
	}

	req, err = http.NewRequestWithContext(ctx, "PUT", respBody.UploadURL, bytes.NewReader(encryptedObj))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to upload object to S3, status code: %d", resp.StatusCode)
	}

	// Update timestamps in the original object
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

// 1. Send a GET request to the API with the given ID
// 2. Parse the API response
// 3.
func (c *APIClient) Get(id string) (any, error) {

}
