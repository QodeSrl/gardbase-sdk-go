package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase/pkg/api/objects"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

type APIClient struct {
	APIEndpoint string

	httpClient *http.Client
}

func NewAPIClient(apiEndpoint string, tenantID string, apiKey string, httpClient *http.Client) *APIClient {
	return &APIClient{
		APIEndpoint: apiEndpoint + "/api",
		httpClient:  httpClient,
	}
}

// Put encrypts a JSON object and its indexed fields, creates a remote object record, and uploads the encrypted object payload.
func (c *APIClient) Put(ctx context.Context, values map[string]any, indexes map[string]any, dek crypto.GeneratedDEK, schemaName string, tableHash string) (objects.CreateObjectResponse, error) {
	// Encrypt object with DEK
	objBytes, err := json.Marshal(values)
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
	}
	encryptedObj, err := crypto.EncryptObjectProbabilistic(objBytes, dek.PlaintextDEK)
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrEncryption, err)
	}

	// Encrypt indexes with DEK
	encryptedIndexesB64 := make(map[string]string, len(indexes))
	for k, v := range indexes {
		indexBytes, err := json.Marshal(v)
		if err != nil {
			return objects.CreateObjectResponse{}, fmt.Errorf("%w: (index %s) %v", errors.ErrValidation, k, err)
		}
		context := fmt.Sprintf("%s:%s", schemaName, k)
		encryptedIndex, err := crypto.EncryptObjectDeterministic(indexBytes, context, dek.PlaintextDEK)
		if err != nil {
			return objects.CreateObjectResponse{}, fmt.Errorf("%w: (index %s) %v", errors.ErrEncryption, k, err)
		}
		encryptedIndexesB64[k] = base64.StdEncoding.EncodeToString(encryptedIndex)
	}

	reqBody, err := json.Marshal(objects.CreateObjectRequest{
		BlobSize:           int64(len(encryptedObj)),
		KMSEncryptedDEK:    base64.StdEncoding.EncodeToString(dek.KMSEncryptedDEK),
		MasterEncryptedDEK: base64.StdEncoding.EncodeToString(dek.MasterKeyEncryptedDEK),
		DEKNonce:           base64.StdEncoding.EncodeToString(dek.MasterKeyNonce),
		Indexes:            encryptedIndexesB64,
		Sensitivity:        "medium",
	})
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
	}

	// Create remote object record
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/"+tableHash, bytes.NewReader(reqBody))
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrNetwork, err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return objects.CreateObjectResponse{}, fmt.Errorf("%w: unauthorized access when creating object", errors.ErrUnauthorized)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return objects.CreateObjectResponse{}, fmt.Errorf("%w: rate limit exceeded when creating object", errors.ErrRateLimited)
		}
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: failed to create object, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	defer resp.Body.Close()

	respBody := objects.CreateObjectResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	// Upload encrypted object
	req, err = http.NewRequestWithContext(ctx, "PUT", respBody.UploadURL, bytes.NewReader(encryptedObj))
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: %w", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return objects.CreateObjectResponse{}, fmt.Errorf("%w: unauthorized access to upload URL", errors.ErrUnauthorized)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return objects.CreateObjectResponse{}, fmt.Errorf("%w: rate limit exceeded when uploading encrypted object", errors.ErrRateLimited)
		}
		return objects.CreateObjectResponse{}, fmt.Errorf("%w: failed to upload encrypted object, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}

	return respBody, nil
}

type GetObjectResult struct {
	EncryptedObj     []byte
	KMSWrappedDEK    []byte
	MasterWrappedDEK []byte
	DEKNonce         []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Get retrieves an encrypted object by its ID and returns the encrypted payload.
func (c *APIClient) Get(ctx context.Context, tableHash string, id string) (GetObjectResult, error) {
	// Call the API and get the object metadata and S3 URL
	req, err := http.NewRequestWithContext(ctx, "GET", c.APIEndpoint+"/objects/"+tableHash+"/"+id, nil)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return GetObjectResult{}, fmt.Errorf("%w: object with ID %s not found", errors.ErrNotFound, id)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return GetObjectResult{}, fmt.Errorf("%w: unauthorized access to object with ID %s", errors.ErrUnauthorized, id)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return GetObjectResult{}, fmt.Errorf("%w: rate limit exceeded when accessing object with ID %s", errors.ErrRateLimited, id)
		}
		return GetObjectResult{}, fmt.Errorf("%w: failed to get object, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	respBody := objects.GetObjectResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	// Download the encrypted object from S3
	req, err = http.NewRequestWithContext(ctx, "GET", respBody.GetURL, nil)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return GetObjectResult{}, fmt.Errorf("%w: object with ID %s not found in S3", errors.ErrNotFound, id)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return GetObjectResult{}, fmt.Errorf("%w: unauthorized access to object with ID %s in S3", errors.ErrUnauthorized, id)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return GetObjectResult{}, fmt.Errorf("%w: rate limit exceeded when accessing object with ID %s in S3", errors.ErrRateLimited, id)
		}
		return GetObjectResult{}, fmt.Errorf("%w: failed to get object from S3, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	KMSWrappedDEK, err := base64.StdEncoding.DecodeString(respBody.KMSWrappedDEK)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	MasterWrappedDEK, err := base64.StdEncoding.DecodeString(respBody.MasterWrappedDEK)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	DEKNonce, err := base64.StdEncoding.DecodeString(respBody.DEKNonce)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	// Build and return the result
	return GetObjectResult{
		EncryptedObj:     buf.Bytes(),
		KMSWrappedDEK:    KMSWrappedDEK,
		MasterWrappedDEK: MasterWrappedDEK,
		DEKNonce:         DEKNonce,
		CreatedAt:        respBody.CreatedAt,
		UpdatedAt:        respBody.UpdatedAt,
	}, nil
}
