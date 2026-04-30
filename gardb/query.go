package gardb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qodesrl/gardbase-sdk-go/gardb/errors"
	"github.com/qodesrl/gardbase-sdk-go/internal"
	"github.com/qodesrl/gardbase/pkg/api/objects"
	"github.com/qodesrl/gardbase/pkg/crypto"
)

type QueryBuilder[T GardbObject] struct {
	schema       *GardbSchema[T]
	ctx          context.Context
	indexOptions []*objects.IndexName // will be fixed to the first index that matches the query conditions at execute time
	hashValue    any
	rangeValue   any
	rangeOp      objects.QueryOperator
	limit        int
	cursor       *string
	scanForward  bool
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

func (qb *QueryBuilder[T]) WhereHash(field string, cond QueryCondition) *QueryBuilder[T] {
	if cond.Op != objects.QueryEq {
		panic("Where only supports equality conditions. Use WhereRange for range conditions.")
	}
	indexes, exists := qb.schema.findIndexByHash(field)
	if !exists {
		panic(fmt.Sprintf("no index found for hash key: %s", field))
	}
	qb.indexOptions = append(qb.indexOptions, indexes...)
	qb.hashValue = cond.Value
	qb.rangeOp = objects.QueryEq
	return qb
}

func (qb *QueryBuilder[T]) WhereRange(field string, cond QueryCondition) *QueryBuilder[T] {
	if len(qb.indexOptions) == 0 {
		panic("WhereHash must be called before WhereRange to specify the hash key")
	}
	index, exists := qb.schema.findIndexByRange(qb.indexOptions, field)
	if !exists {
		panic(fmt.Sprintf("no index found for range key: %s", field))
	}
	qb.indexOptions = []*objects.IndexName{index}
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

	if err := qb.schema.ensureTableIEK(qb.ctx); err != nil {
		return nil, fmt.Errorf("%s: failed to ensure table IEK: %w", op, err)
	}

	var indexName *objects.IndexName
	if len(qb.indexOptions) == 0 {
		return nil, fmt.Errorf("%s: no index specified for query", op)
	} else if len(qb.indexOptions) == 1 {
		indexName = qb.indexOptions[0]
	} else {
		if qb.rangeValue == nil {
			indexName = &objects.IndexName{
				HashField:  qb.indexOptions[0].HashField,
				RangeField: nil,
			}
		} else {
			for _, idx := range qb.indexOptions {
				if idx.RangeField != nil && *idx.RangeField == *qb.indexOptions[0].RangeField {
					indexName = idx
					break
				}
			}
			if indexName == nil {
				return nil, fmt.Errorf("%s: no suitable index found for range query", op)
			}
		}
	}

	index := internal.Index{
		Name:       *indexName,
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
