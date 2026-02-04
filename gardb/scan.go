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

type ScanInput struct {
	Limit     int
	NextToken *string
}

func (s *Schema) Scan(ctx context.Context, obj any, config *ScanInput) error {
	const op = "Schema.Scan"

	if !validatePtrToSliceOfStructsWithGardbMeta(obj) {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: expected pointer to struct with GardbMeta field", errors.ErrValidation),
		}
	}

	data, err := s.client.apiClient.Scan(ctx, s.tableHash, config.Limit, config.NextToken)
	if err != nil {
		if internal.IsContextError(err) {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	dekObjs := make([]internal.DecryptDEKObject, len(data))
	for i, item := range data {
		dekObjs[i] = internal.DecryptDEKObject{
			ObjectID: item.ObjectID,
			DEKB64:   base64.StdEncoding.EncodeToString(item.KMSWrappedDEK),
		}
	}
	deks, err := s.client.enclaveClient.DecryptDEKs(ctx, dekObjs)
	if err != nil {
		if internal.IsContextError(err) {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
			}
		}
		return &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	slicePtr := reflect.ValueOf(obj)
	sliceVal := slicePtr.Elem()
	elemType := sliceVal.Type().Elem()

	for i, item := range data {
		if deks[i].Error != nil {
			continue
		}

		decryptedObjBytes, err := crypto.DecryptObjectProbabilistic(item.EncryptedObj, deks[i].DEK)
		if err != nil {
			if internal.IsContextError(err) {
				return &errors.Error{
					Op:  op,
					Err: fmt.Errorf("%w: %w", errors.ErrCancelledOrTimedOut, err),
				}
			}
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to decrypt object: %v", errors.ErrEncryption, err),
			}
		}

		elemPtr := reflect.New(elemType)

		if err = json.Unmarshal(decryptedObjBytes, elemPtr.Interface()); err != nil {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to unmarshal object: %v", errors.ErrEncryption, err),
			}
		}

		// Update GardbMeta fields
		elemVal := elemPtr.Elem()
		metaField := elemVal.FieldByName("GardbMeta")
		if !metaField.IsValid() {
			return &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: GardbMeta field not found", errors.ErrValidation),
			}
		}

		meta := metaField.Interface().(GardbMeta)
		meta.schema = s
		meta.ID = item.ObjectID
		meta.CreatedAt = item.CreatedAt
		meta.UpdatedAt = item.UpdatedAt

		metaField.Set(reflect.ValueOf(meta))

		sliceVal.Set(reflect.Append(sliceVal, elemVal))
	}

	return nil
}
