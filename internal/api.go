package internal

import (
	"bytes"
	"context"
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
func (c *APIClient) Put(ctx context.Context, values map[string]any, indexes []Index, dek crypto.GeneratedDEK, iek []byte, tableHash string, objectId string, currentVersion int32) (PutObjectResult, error) {
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
	encryptedIndexes, err := EncryptIndexes(indexes, tableHash, iek)
	if err != nil {
		return PutObjectResult{}, err
	}

	blobSize := int64(len(encryptedObj))

	if blobSize < 100*1024 { // For smaller objects, include the encrypted blob directly in the request
		reqBody, err := json.Marshal(objects.PutObjectRequest{
			ObjectID:           objectId,
			TableHash:          tableHash,
			EncryptedBlob:      encryptedObj,
			KMSEncryptedDEK:    dek.KMSEncryptedDEK,
			MasterEncryptedDEK: dek.MasterKeyEncryptedDEK,
			DEKNonce:           dek.MasterKeyNonce,
			Indexes:            encryptedIndexes,
			Sensitivity:        "medium",
			Version:            currentVersion + 1,
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
			ObjectID:  objectId,
			TableHash: tableHash,
			BlobSize:  blobSize,
			Version:   currentVersion + 1,
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
			KMSEncryptedDEK:    dek.KMSEncryptedDEK,
			MasterEncryptedDEK: dek.MasterKeyEncryptedDEK,
			DEKNonce:           dek.MasterKeyNonce,
			Indexes:            encryptedIndexes,
			Sensitivity:        "medium",
			Version:            currentVersion + 1,
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
		if resp.StatusCode == http.StatusGone {
			return GetObjectResult{}, fmt.Errorf("%w: object with ID %s has been deleted", errors.ErrDeleted, id)
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

	if respBody.GetURL == "" {
		encryptedObj = respBody.EncryptedBlob
	} else {
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
	}

	// Build and return the result
	return GetObjectResult{
		ObjectID:         id,
		EncryptedObj:     encryptedObj,
		KMSWrappedDEK:    respBody.KMSWrappedDEK,
		MasterWrappedDEK: respBody.MasterWrappedDEK,
		DEKNonce:         respBody.DEKNonce,
		CreatedAt:        respBody.CreatedAt,
		UpdatedAt:        respBody.UpdatedAt,
		Version:          respBody.Version,
	}, nil
}

type QueryResult struct {
	NextToken *string
	Count     int
	Objects   []GetObjectResult
}

func (c *APIClient) Scan(ctx context.Context, tableHash string, limit int, nextToken *string) (QueryResult, error) {
	// Call the API and get the list of objects
	reqBody, err := json.Marshal(objects.ScanRequest{
		TableHash: tableHash,
		Limit:     limit,
		NextToken: nextToken,
	})
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/scan", bytes.NewReader(reqBody))
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return QueryResult{}, fmt.Errorf("%w: unauthorized access", errors.ErrUnauthorized)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return QueryResult{}, fmt.Errorf("%w: rate limit exceeded when scanning table", errors.ErrRateLimited)
		}
		return QueryResult{}, fmt.Errorf("%w: failed to scan table, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	respBody := objects.ScanResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	var results QueryResult

	for _, obj := range respBody.Objects {
		getObjResult, err := c.ensureObjectBlob(ctx, obj)
		if err != nil {
			return QueryResult{}, fmt.Errorf("%w: failed to get object blob for object ID %s: %v", errors.ErrNetwork, obj.ObjectID, err)
		}
		results.Objects = append(results.Objects, *getObjResult)
	}

	results.Count = respBody.Count
	results.NextToken = respBody.NextToken

	return results, nil
}

func (c *APIClient) Query(ctx context.Context, tableHash string, iek []byte, index Index, rangeOp objects.QueryOperator, limit int, nextToken *string, scanForward bool) (QueryResult, error) {

	queryReq := objects.QueryRequest{
		TableHash:   tableHash,
		RangeOp:     rangeOp,
		Limit:       limit,
		NextToken:   nextToken,
		ScanForward: scanForward,
	}

	if rangeOp == objects.RangeBetween {
		rangeValues, ok := index.RangeValue.([2]any)
		if !ok {
			return QueryResult{}, fmt.Errorf("%w: for RangeBetween operator, RangeValue must be of type [2]any", errors.ErrValidation)
		}
		emptyIdx, betweenRange, err := EncryptIndexForBetweenRange(index, tableHash, rangeValues, iek)
		if err != nil {
			return QueryResult{}, err
		}
		queryReq.Index = emptyIdx
		queryReq.BetweenRange = betweenRange
	} else {
		// encrypt index with IEK
		idxNoObjectID, err := EncryptIndex(index, tableHash, iek)
		if err != nil {
			return QueryResult{}, err
		}
		queryReq.Index = idxNoObjectID
		queryReq.BetweenRange = [2][]byte{nil, nil}
	}

	// Call the API
	reqBody, err := json.Marshal(queryReq)
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/query", bytes.NewReader(reqBody))
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return QueryResult{}, fmt.Errorf("%w: unauthorized access when querying table", errors.ErrUnauthorized)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return QueryResult{}, fmt.Errorf("%w: rate limit exceeded when querying table", errors.ErrRateLimited)
		}
		return QueryResult{}, fmt.Errorf("%w: failed to query table, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	respBody := objects.QueryResponse{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return QueryResult{}, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}

	var results QueryResult
	for _, obj := range respBody.Objects {
		getObjResult, err := c.ensureObjectBlob(ctx, obj)
		if err != nil {
			return QueryResult{}, fmt.Errorf("%w: failed to get object blob for object ID %s: %v", errors.ErrNetwork, obj.ObjectID, err)
		}
		results.Objects = append(results.Objects, *getObjResult)
	}
	results.Count = respBody.Count
	results.NextToken = respBody.NextToken

	return results, nil
}

func (c *APIClient) ensureObjectBlob(ctx context.Context, obj objects.ResultObject) (*GetObjectResult, error) {
	var encryptedObj []byte

	if obj.GetURL == "" {
		encryptedObj = obj.EncryptedBlob
	} else {
		// Download the encrypted object from S3
		req, err := http.NewRequestWithContext(ctx, "GET", obj.GetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errors.ErrValidation, err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("%w: object with ID %s not found in S3", errors.ErrNotFound, obj.ObjectID)
			}
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("%w: unauthorized access to object with ID %s in S3", errors.ErrUnauthorized, obj.ObjectID)
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, fmt.Errorf("%w: rate limit exceeded when accessing object with ID %s in S3", errors.ErrRateLimited, obj.ObjectID)
			}
			return nil, fmt.Errorf("%w: failed to get object from S3, status code: %d", errors.ErrNetwork, resp.StatusCode)
		}

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errors.ErrNetwork, err)
		}
		encryptedObj = buf.Bytes()
	}

	// Build and return the result
	return &GetObjectResult{
		ObjectID:         obj.ObjectID,
		EncryptedObj:     encryptedObj,
		KMSWrappedDEK:    obj.KMSWrappedDEK,
		MasterWrappedDEK: obj.MasterWrappedDEK,
		DEKNonce:         obj.DEKNonce,
		CreatedAt:        obj.CreatedAt,
		UpdatedAt:        obj.UpdatedAt,
	}, nil
}

func (c *APIClient) Delete(ctx context.Context, tableHash string, objectId string) error {
	reqBody, err := json.Marshal(objects.DeleteObjectRequest{
		TableHash: tableHash,
		ObjectID:  objectId,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/delete", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: object with ID %s not found", errors.ErrNotFound, objectId)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: unauthorized access to object with ID %s", errors.ErrUnauthorized, objectId)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("%w: rate limit exceeded when deleting object with ID %s", errors.ErrRateLimited, objectId)
		}
		return fmt.Errorf("%w: failed to delete object, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	return nil
}

func (c *APIClient) Recover(ctx context.Context, tableHash string, objectId string) error {
	reqBody, err := json.Marshal(objects.RecoverObjectRequest{
		TableHash: tableHash,
		ObjectID:  objectId,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIEndpoint+"/objects/recover", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("%w: %v", errors.ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: object with ID %s not found", errors.ErrNotFound, objectId)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: unauthorized access to object with ID %s", errors.ErrUnauthorized, objectId)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("%w: rate limit exceeded when recovering object with ID %s", errors.ErrRateLimited, objectId)
		}
		return fmt.Errorf("%w: failed to recover object, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	return nil
}
