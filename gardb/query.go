package gardb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase/pkg/api/objects"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

type QueryBuilder[T GardbObject] struct {
	schema      *GardbSchema[T]
	ctx         context.Context
	hashKey     string
	hashValue   any
	rangeKey    string
	rangeValue  any
	rangeOp     objects.QueryOperator
	limit       int
	cursor      *string
	scanForward bool
}

type QueryOutput[T GardbObject] struct {
	Items      []T
	Count      int
	NextCursor *string
}

func (s *GardbSchema[T]) Query(ctx context.Context) *QueryBuilder[T] {
	return &QueryBuilder[T]{
		schema:      s,
		ctx:         ctx,
		scanForward: true,
		limit:       100,
	}
}

func (qb *QueryBuilder[T]) Where(field string, cond QueryCondition) *QueryBuilder[T] {
	if cond.Op != objects.QueryEq {
		panic("Where only supports equality conditions. Use WhereRange for range conditions.")
	}
	hashExists := qb.schema.containsHashKey(field)
	if !hashExists {
		panic(fmt.Sprintf("no index found for field: %s", field))
	}

	qb.hashKey = field
	qb.hashValue = cond.Value
	return qb
}

func (qb *QueryBuilder[T]) WhereRange(field string, cond QueryCondition) *QueryBuilder[T] {
	hashAndRangeExists := qb.schema.containsHashAndRangeKey(qb.hashKey, field)
	if !hashAndRangeExists {
		panic(fmt.Sprintf("no index found for hash key: %s and range key: %s", qb.hashKey, field))
	}
	qb.rangeKey = field
	qb.rangeValue = cond.Value
	qb.rangeOp = cond.Op
	return qb
}

func (qb *QueryBuilder[T]) Limit(n int) *QueryBuilder[T] {
	if n <= 0 {
		panic("limit must be greater than 0")
	}
	qb.limit = n
	return qb
}

func (qb *QueryBuilder[T]) StartFrom(cursor string) *QueryBuilder[T] {
	qb.cursor = &cursor
	return qb
}

func (qb *QueryBuilder[T]) OrderBy(ascending bool) *QueryBuilder[T] {
	qb.scanForward = ascending
	return qb
}

func (qb *QueryBuilder[T]) Execute() (*QueryOutput[T], error) {
	const op = "Schema.Query.Execute"

	if qb.hashKey == "" {
		return nil, fmt.Errorf("%s: hash key must be specified", op)
	}

	if err := qb.schema.ensureTableIEK(qb.ctx); err != nil {
		return nil, fmt.Errorf("%s: failed to ensure table IEK: %w", op, err)
	}

	indexName := objects.IndexName{
		HashField:  qb.hashKey,
		RangeField: &qb.rangeKey,
	}
	if qb.rangeKey == "" {
		indexName.RangeField = nil
	}
	index := internal.Index{
		Name:       indexName,
		HashValue:  qb.hashValue,
		RangeValue: qb.rangeValue,
	}
	data, err := qb.schema.client.apiClient.Query(qb.ctx, qb.schema.tableHash, qb.schema.tableIEK, index, qb.rangeOp, qb.limit, qb.cursor, qb.scanForward)
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
	deks, err := qb.schema.client.enclaveClient.DecryptDEKs(qb.ctx, dekObjs)
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

		decryptObjBytes, err := crypto.DecryptObjectProbabilistic(item.EncryptedObj, deks[i].DEK)
		if err != nil {
			if internal.IsContextError(err) {
				return nil, &errors.Error{
					Op:  op,
					Err: fmt.Errorf("%w: %v", errors.ErrCancelledOrTimedOut, err),
				}
			}
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to decrypt object: %v", errors.ErrEncryption, err),
			}
		}

		var raw map[string]any
		if err = json.Unmarshal(decryptObjBytes, &raw); err != nil {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to unmarshal object: %v", errors.ErrEncryption, err),
			}
		}
		obj := qb.schema.newInstance()
		if err = qb.schema.populate(obj, raw); err != nil {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: failed to populate object: %v", errors.ErrEncryption, err),
			}
		}

		meta := obj.getGardbMeta()
		meta.ID = item.ObjectID
		meta.CreatedAt = item.CreatedAt
		meta.UpdatedAt = item.UpdatedAt

		results = append(results, obj)
	}

	return &QueryOutput[T]{
		Items:      results,
		Count:      data.Count,
		NextCursor: data.NextToken,
	}, nil
}
