package gardb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

type ScanInput struct {
	Limit  int
	Cursor *string
}

type ScanOutput[T GardbObject] struct {
	Items      []T
	Count      int
	Limit      int
	NextCursor *string
}

// Scan retrieves a list of decrypted objects from the database based on the provided scan configuration.
//
// It performs a scan operation using the API client to fetch encrypted object payloads, decrypts the Data Encryption Keys (DEKs)
// through the enclave client, decrypts each object using the corresponding plaintext DEK, and unmarshals the results
// into a slice of the specified type T. Each object's metadata (ID, CreatedAt, UpdatedAt) is populated from the
// retrieved data.
//
// Parameters:
//   - ctx: The context for managing request cancellation and timeout
//   - config: The configuration for the scan operation, including limit and pagination token
//
// Returns:
//   - A slice of objects of type T containing the decrypted and unmarshalled data
//   - A ScanOutput containing the next pagination token if more results are available
//   - An error if any step of the retrieval, decryption, or unmarshalling process fails, or if the context is cancelled/times out
func (s *GardbSchema[T]) Scan(ctx context.Context, config *ScanInput) (*ScanOutput[T], error) {
	const op = "Schema.Scan"

	if config == nil {
		config = &ScanInput{
			Limit: 100,
		}
	}

	data, err := s.client.apiClient.Scan(ctx, s.tableHash, config.Limit, config.Cursor)
	if err != nil {
		if internal.IsContextError(err) {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return nil, &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	dekObjs := make([]internal.DecryptDEKObject, len(data.Objects))
	for i, item := range data.Objects {
		dekObjs[i] = internal.DecryptDEKObject{
			ObjectID: item.ObjectID,
			DEK:      item.KMSWrappedDEK,
		}
	}
	deks, err := s.client.enclaveClient.DecryptDEKs(ctx, dekObjs)
	if err != nil {
		if internal.IsContextError(err) {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return nil, &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	results := make([]T, 0, len(data.Objects))

	for i, item := range data.Objects {
		if deks[i].Error != nil {
			continue
		}

		decryptedObjBytes, err := crypto.DecryptObjectProbabilistic(item.EncryptedObj, deks[i].DEK)
		if err != nil {
			if internal.IsContextError(err) {
				return nil, &errors.Error{
					Op:  op,
					Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
				}
			}
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to decrypt object: %v", errors.ErrEncryption, err),
			}
		}

		var raw map[string]any
		if err = json.Unmarshal(decryptedObjBytes, &raw); err != nil {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to unmarshal object: %v", errors.ErrEncryption, err),
			}
		}
		obj := s.newInstance()
		if err = s.populate(obj, raw); err != nil {
			return nil, &errors.Error{
				Op:  op,
				Err: err,
			}
		}

		meta := obj.getGardbMeta()
		meta.ID = item.ObjectID
		meta.CreatedAt = item.CreatedAt
		meta.UpdatedAt = item.UpdatedAt

		results = append(results, obj)
	}

	return &ScanOutput[T]{
		Items:      results,
		Limit:      config.Limit,
		NextCursor: data.NextToken,
		Count:      data.Count,
	}, nil
}
