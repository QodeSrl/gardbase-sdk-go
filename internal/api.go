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

type PutObjectResult struct {
	ObjectID  string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int32
}

// Put encrypts a JSON object and its indexed fields, creates a remote object record, and uploads the encrypted object payload.
func (c *APIClient) Put(ctx context.Context, values map[string]any, indexes map[string]any, dek crypto.GeneratedDEK, iek []byte, schemaName string, tableHash string) (PutObjectResult, error) {
	// Encrypt object with DEK
	objBytes, err := json.Marshal(values)
	if err != nil {
		return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
	}
	encryptedObj, err := crypto.EncryptObjectProbabilistic(objBytes, dek.PlaintextDEK)
	if err != nil {
		return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrEncryption, err)
	}

	// Encrypt indexes with IEK
	encryptedIndexes := make(map[string][]byte, len(indexes))
	for k, v := range indexes {
		indexBytes, err := json.Marshal(v)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: (index %s) %v", errors.ErrValidation, k, err)
		}
		context := fmt.Sprintf("%s:%s", schemaName, k)
		encryptedIndex, err := crypto.EncryptObjectDeterministic(indexBytes, context, iek)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: (index %s) %v", errors.ErrEncryption, k, err)
		}
		encryptedIndexes[k] = encryptedIndex
	}

	blobSize := int64(len(encryptedObj))

	if blobSize < 100*1024 { // For smaller objects, include the encrypted blob directly in the request
		reqBody, err := json.Marshal(objects.PutObjectRequest{
			TableHash:          tableHash,
			EncryptedBlob:      base64.StdEncoding.EncodeToString(encryptedObj),
			KMSEncryptedDEK:    base64.StdEncoding.EncodeToString(dek.KMSEncryptedDEK),
			MasterEncryptedDEK: base64.StdEncoding.EncodeToString(dek.MasterKeyEncryptedDEK),
			DEKNonce:           base64.StdEncoding.EncodeToString(dek.MasterKeyNonce),
			Indexes:            encryptedIndexes,
			Sensitivity:        "medium",
			Version:            1,
		})
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
		}

		// Upload object
		req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/put", bytes.NewReader(reqBody))
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrNetwork, err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return PutObjectResult{}, fmt.Errorf("%w: unauthorized access when creating object", errors.ErrUnauthorized)
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return PutObjectResult{}, fmt.Errorf("%w: rate limit exceeded when creating object", errors.ErrRateLimited)
			}
			return PutObjectResult{}, fmt.Errorf("%w: failed to create object, status code: %d", errors.ErrNetwork, resp.StatusCode)
		}
		defer resp.Body.Close()

		respBody := objects.PutObjectResponse{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
		return PutObjectResult{
			ObjectID:  respBody.ObjectID,
			CreatedAt: respBody.CreatedAt,
			UpdatedAt: respBody.UpdatedAt,
			Version:   respBody.Version,
		}, nil
	} else {
		// Request pre-signed URL for uploading large object
		reqBody, err := json.Marshal(objects.RequestPutLargeObjectRequest{
			TableHash: tableHash,
			BlobSize:  blobSize,
			Version:   1,
		})
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/request-put-large", bytes.NewReader(reqBody))
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrNetwork, err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return PutObjectResult{}, fmt.Errorf("%w: failed to request pre-signed URL for large object, status code: %d", errors.ErrNetwork, resp.StatusCode)
		}
		defer resp.Body.Close()
		respBody := objects.RequestPutLargeObjectResponse{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
		req, err = http.NewRequestWithContext(ctx, "PUT", respBody.UploadURL, bytes.NewReader(encryptedObj))
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrValidation, err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %w", errors.ErrNetwork, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return PutObjectResult{}, fmt.Errorf("%w: unauthorized access to upload URL", errors.ErrUnauthorized)
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return PutObjectResult{}, fmt.Errorf("%w: rate limit exceeded when uploading encrypted object", errors.ErrRateLimited)
			}
			return PutObjectResult{}, fmt.Errorf("%w: failed to upload encrypted object, status code: %d", errors.ErrNetwork, resp.StatusCode)
		}
		reqBody, err = json.Marshal(objects.ConfirmPutLargeObjectRequest{
			ObjectID:           respBody.ObjectID,
			TableHash:          tableHash,
			KMSEncryptedDEK:    base64.StdEncoding.EncodeToString(dek.KMSEncryptedDEK),
			MasterEncryptedDEK: base64.StdEncoding.EncodeToString(dek.MasterKeyEncryptedDEK),
			DEKNonce:           base64.StdEncoding.EncodeToString(dek.MasterKeyNonce),
			Indexes:            encryptedIndexes,
			Sensitivity:        "medium",
			Version:            1,
		})
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
		}
		req, err = http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/confirm-put-large", bytes.NewReader(reqBody))
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return PutObjectResult{}, fmt.Errorf("%w: unauthorized access when confirming object creation", errors.ErrUnauthorized)
			}
			if resp.StatusCode == http.StatusTooManyRequests {

				return PutObjectResult{}, fmt.Errorf("%w: rate limit exceeded when confirming object creation", errors.ErrRateLimited)
			}
			return PutObjectResult{}, fmt.Errorf("%w: failed to confirm object creation, status code: %d", errors.ErrNetwork, resp.StatusCode)
		}
		confirmRespBody := objects.ConfirmPutLargeObjectResponse{}
		err = json.NewDecoder(resp.Body).Decode(&confirmRespBody)
		if err != nil {
			return PutObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
		return PutObjectResult{
			ObjectID:  respBody.ObjectID,
			CreatedAt: confirmRespBody.CreatedAt,
			UpdatedAt: confirmRespBody.UpdatedAt,
			Version:   confirmRespBody.Version,
		}, nil
	}
}

type GetObjectResult struct {
	ObjectID         string
	EncryptedObj     []byte
	KMSWrappedDEK    []byte
	MasterWrappedDEK []byte
	DEKNonce         []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int32
}

// Get retrieves an encrypted object by its ID and returns the encrypted payload.
func (c *APIClient) Get(ctx context.Context, tableHash string, id string) (GetObjectResult, error) {
	reqBody, err := json.Marshal(objects.GetObjectRequest{
		TableHash: tableHash,
		ObjectID:  id,
	})
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	// Call the API and get the object metadata and S3 URL
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/get", bytes.NewReader(reqBody))
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

	var encryptedObj []byte

	if respBody.GetURL != "" {
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
		encryptedObj = buf.Bytes()
	} else {
		encryptedObj, err = base64.StdEncoding.DecodeString(respBody.EncryptedBlob)
		if err != nil {
			return GetObjectResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
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
		ObjectID:         id,
		EncryptedObj:     encryptedObj,
		KMSWrappedDEK:    KMSWrappedDEK,
		MasterWrappedDEK: MasterWrappedDEK,
		DEKNonce:         DEKNonce,
		CreatedAt:        respBody.CreatedAt,
		UpdatedAt:        respBody.UpdatedAt,
		Version:          respBody.Version,
	}, nil
}

type ScanResult struct {
	NextToken *string
	Results   []GetObjectResult
}

func (c *APIClient) Scan(ctx context.Context, tableHash string, limit int, nextToken *string) (ScanResult, error) {
	// Call the API and get the list of objects
	reqBody, err := json.Marshal(objects.ScanRequest{
		TableHash: tableHash,
		Limit:     limit,
		NextToken: nextToken,
	})
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/scan", bytes.NewReader(reqBody))
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return ScanResult{}, fmt.Errorf("%w: unauthorized access", errors.ErrUnauthorized)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return ScanResult{}, fmt.Errorf("%w: rate limit exceeded when scanning table", errors.ErrRateLimited)
		}
		return ScanResult{}, fmt.Errorf("%w: failed to scan table, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	respBody := objects.ScanResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	var results ScanResult

	for _, obj := range respBody.Objects {
		var encryptedObj []byte

		if obj.GetURL != "" {
			// Download the encrypted object from S3
			req, err = http.NewRequestWithContext(ctx, "GET", obj.GetURL, nil)
			if err != nil {
				return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
			}
			req.Header.Set("Content-Type", "application/octet-stream")
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				if resp.StatusCode == http.StatusNotFound {
					return ScanResult{}, fmt.Errorf("%w: object with ID %s not found in S3", errors.ErrNotFound, obj.ObjectID)
				}
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					return ScanResult{}, fmt.Errorf("%w: unauthorized access to object with ID %s in S3", errors.ErrUnauthorized, obj.ObjectID)
				}
				if resp.StatusCode == http.StatusTooManyRequests {
					return ScanResult{}, fmt.Errorf("%w: rate limit exceeded when accessing object with ID %s in S3", errors.ErrRateLimited, obj.ObjectID)
				}
				return ScanResult{}, fmt.Errorf("%w: failed to get object from S3, status code: %d", errors.ErrNetwork, resp.StatusCode)
			}

			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(resp.Body)
			if err != nil {
				return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
			}
			encryptedObj = buf.Bytes()
		} else {
			encryptedObj, err = base64.StdEncoding.DecodeString(obj.EncryptedBlob)
			if err != nil {
				return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
			}
		}

		KMSWrappedDEK, err := base64.StdEncoding.DecodeString(obj.KMSWrappedDEK)
		if err != nil {
			return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}

		MasterWrappedDEK, err := base64.StdEncoding.DecodeString(obj.MasterWrappedDEK)
		if err != nil {
			return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}

		DEKNonce, err := base64.StdEncoding.DecodeString(obj.DEKNonce)
		if err != nil {
			return ScanResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}

		// Build and return the result
		results.Results = append(results.Results, GetObjectResult{
			ObjectID:         obj.ObjectID,
			EncryptedObj:     encryptedObj,
			KMSWrappedDEK:    KMSWrappedDEK,
			MasterWrappedDEK: MasterWrappedDEK,
			DEKNonce:         DEKNonce,
			CreatedAt:        obj.CreatedAt,
			UpdatedAt:        obj.UpdatedAt,
		})
		results.NextToken = respBody.NextToken
	}

	return results, nil
}
