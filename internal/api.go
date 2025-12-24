package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
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

// Put encrypts a JSON object and its indexed fields, creates a remote object record, and uploads the encrypted object payload.
func (c *APIClient) Put(ctx context.Context, values map[string]any, indexes map[string]any, dek crypto.GeneratedDEK, schema *schema.Schema) (models.CreateObjectResponse, error) {
	// Encrypt object with DEK
	objBytes, err := json.Marshal(values)
	if err != nil {
		return models.CreateObjectResponse{}, err
	}
	encryptedObj, err := crypto.EncryptObjectProbabilistic(objBytes, dek.PlaintextDEK)
	if err != nil {
		return models.CreateObjectResponse{}, err
	}

	// Encrypt indexes with DEK
	encryptedIndexesB64 := make(map[string]string, len(indexes))
	for k, v := range indexes {
		indexBytes, err := json.Marshal(v)
		if err != nil {
			return models.CreateObjectResponse{}, err
		}
		context := fmt.Sprintf("%s:%s", schema.Name(), k)
		encryptedIndex, err := crypto.EncryptObjectDeterministic(indexBytes, context, dek.PlaintextDEK)
		if err != nil {
			return models.CreateObjectResponse{}, err
		}
		encryptedIndexesB64[k] = base64.StdEncoding.EncodeToString(encryptedIndex)
	}

	reqBody, err := json.Marshal(models.CreateObjectRequest{
		EncryptedDEK: base64.StdEncoding.EncodeToString(dek.EncryptedDEK),
		Indexes:      encryptedIndexesB64,
		Sensitivity:  "medium",
	})
	if err != nil {
		return models.CreateObjectResponse{}, err
	}

	// Create remote object record
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects", bytes.NewReader(reqBody))
	if err != nil {
		return models.CreateObjectResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.CreateObjectResponse{}, err
	}
	defer resp.Body.Close()

	respBody := models.CreateObjectResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return models.CreateObjectResponse{}, err
	}

	// Upload encrypted object to S3
	req, err = http.NewRequestWithContext(ctx, "PUT", respBody.UploadURL, bytes.NewReader(encryptedObj))
	if err != nil {
		return models.CreateObjectResponse{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return models.CreateObjectResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return models.CreateObjectResponse{}, fmt.Errorf("failed to upload object to S3, status code: %d", resp.StatusCode)
	}

	return respBody, nil
}

type GetObjectResult struct {
	EncryptedObj []byte
	DEK          []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Get retrieves an encrypted object by its ID and returns the encrypted payload.
func (c *APIClient) Get(ctx context.Context, id string) (GetObjectResult, error) {
	// Call the API and get the object metadata and S3 URL
	req, err := http.NewRequestWithContext(ctx, "GET", c.APIEndpoint+"/objects/"+id, nil)
	if err != nil {
		return GetObjectResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GetObjectResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GetObjectResult{}, fmt.Errorf("failed to get object, status code: %d", resp.StatusCode)
	}
	respBody := models.GetObjectResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return GetObjectResult{}, err
	}

	// Download the encrypted object from S3
	req, err = http.NewRequestWithContext(ctx, "GET", respBody.GetURL, nil)
	if err != nil {
		return GetObjectResult{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return GetObjectResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GetObjectResult{}, fmt.Errorf("failed to get object from S3, status code: %d", resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return GetObjectResult{}, err
	}
	dek, err := base64.StdEncoding.DecodeString(respBody.EncryptedDEK)
	if err != nil {
		return GetObjectResult{}, err
	}

	// Build and return the result
	return GetObjectResult{
		EncryptedObj: buf.Bytes(),
		DEK:          dek,
		CreatedAt:    respBody.CreatedAt,
		UpdatedAt:    respBody.UpdatedAt,
	}, nil
}
