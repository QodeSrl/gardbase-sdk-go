package gardb

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/qodesrl/gardbase-sdk-go/gardb/errors"
	"github.com/qodesrl/gardbase-sdk-go/internal"
	"github.com/qodesrl/gardbase-sdk-go/schema"
	"github.com/qodesrl/gardbase/pkg/api/objects"
)

type GardbSchema[T GardbObject] struct {
	name       string
	tableHash  string
	tableIEK   []byte
	fields     Model
	indexes    Indexes
	timeFields []string
	client     *Client
	typ        reflect.Type // the struct type T points to
}

type GardbObject interface {
	getGardbMeta() *GardbMeta
}

type GardbBase struct {
	GardbMeta
}

func (s *GardbSchema[T]) ensureTableIEK(ctx context.Context) error {
	const op = "Schema.ensureTableIEK"

	if s.tableIEK != nil {
		return nil
	}

	iek, err := s.client.enclaveClient.GetTableIEK(ctx, s.name)
	if err != nil {
		return &errors.Error{
			Op:  op,
			Err: err,
		}
	}

	s.tableIEK = iek

	return nil
}

func (g *GardbBase) getGardbMeta() *GardbMeta {
	return &g.GardbMeta
}

type GardbMeta struct {
	ID        string
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Model map[string]*schema.Field // schema.String(), schema.Int(), etc.

type Indexes []*objects.IndexName

// Name returns the name of the schema
func (s *GardbSchema[T]) Name() string {
	return s.name
}

func (s *GardbSchema[T]) newInstance() T {
	return reflect.New(s.typ).Interface().(T)
}

func (s *GardbSchema[T]) findIndexByHash(field string) ([]*objects.IndexName, bool) {
	var matchingIndexes []*objects.IndexName
	for i := range s.indexes {
		idx := s.indexes[i]
		if idx.HashField != field {
			continue
		}
		matchingIndexes = append(matchingIndexes, idx)
	}
	return matchingIndexes, len(matchingIndexes) > 0
}

func (s *GardbSchema[T]) findIndexByRange(options []*objects.IndexName, field string) (*objects.IndexName, bool) {
	for _, idx := range options {
		if idx.RangeField != nil && *idx.RangeField == field {
			return idx, true
		}
	}
	return nil, false
}

func (s *GardbSchema[T]) containsHashKey(field string) bool {
	return slices.ContainsFunc(s.indexes, func(idx *objects.IndexName) bool {
		return idx.HashField == field
	})
}

func (s *GardbSchema[T]) containsHashAndRangeKey(hashField, rangeField string) bool {
	if hashField == "" {
		return false
	}
	return slices.ContainsFunc(s.indexes, func(idx *objects.IndexName) bool {
		return idx.HashField == hashField && idx.RangeField != nil && *idx.RangeField == rangeField
	})
}

// validate checks if the given struct pointer conforms to the schema definition (field names, types, required fields)
func (s *GardbSchema[T]) validate(op string, obj T) error {
	rv := reflect.ValueOf(obj).Elem()
	rt := rv.Type()

	structTags := make(map[string]bool)

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		tag := sf.Tag.Get("gardb")
		if tag == "" {
			continue
		}
		structTags[tag] = true

		// Validate field against schema
		sField, ok := s.fields[tag]
		if !ok {
			return &errors.Error{
				Op:    op,
				Field: tag,
				Err:   fmt.Errorf("%w: field not defined in schema", errors.ErrInvalidSchema),
			}
		}

		val := rv.Field(i)
		if !val.IsZero() && !sField.TypeValidator(val.Interface()) {
			return &errors.Error{
				Op:    op,
				Field: tag,
				Err:   fmt.Errorf("%w: invalid type", errors.ErrInvalidSchema),
			}
		}
	}

	// Check if all schema fields were processed
	for fieldName, field := range s.fields {
		if field.IsRequired() && !structTags[fieldName] {
			return &errors.Error{
				Op:    op,
				Field: fieldName,
				Err:   fmt.Errorf("%w: missing required field", errors.ErrInvalidSchema),
			}
		}
	}

	return nil
}

// extract takes a struct pointer and extracts field values into a values map and an indexes map, applying default values and checking required fields
func (s *GardbSchema[T]) extract(obj T) (values map[string]any, indexes []internal.Index, err error) {
	const op = "Schema.Extract"
	valErrors := &errors.ValidationErrors{Op: op}

	values = make(map[string]any, len(s.fields))
	indexes = make([]internal.Index, 0, len(s.indexes))

	rv := reflect.ValueOf(obj).Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		tag := sf.Tag.Get("gardb")
		if tag == "" {
			continue
		}
		s.fields[tag].ExtractIntoValues(rv.Field(i), &values, valErrors, tag)
	}

	for _, idx := range s.indexes {
		hashVal, ok := values[idx.HashField]
		if !ok {
			valErrors.Add(idx.HashField, fmt.Sprintf("%v: missing hash key for index", errors.ErrValidation), idx.HashField)
			continue
		}
		var rangeVal any
		if idx.RangeField != nil {
			rangeVal, ok = values[*idx.RangeField]
			if !ok {
				valErrors.Add(*idx.RangeField, fmt.Sprintf("%v: missing range key for index", errors.ErrValidation), *idx.RangeField)
				continue
			}
		}
		indexes = append(indexes, internal.Index{
			Name:       *idx,
			HashValue:  hashVal,
			RangeValue: rangeVal,
		})
	}

	if valErrors.HasErrors() {
		return nil, nil, valErrors
	}

	return values, indexes, nil
}

// populate takes a struct pointer and populates its fields from the given raw map, converting types as needed (e.g. time fields)
func (s *GardbSchema[T]) populate(obj T, raw map[string]any) error {
	const op = "Schema.populate"
	rv := reflect.ValueOf(obj).Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		tag := sf.Tag.Get("gardb")
		if tag == "" {
			continue
		}
		rawVal, ok := raw[tag]
		if !ok {
			continue
		}
		schemaField, ok := s.fields[tag]
		if !ok {
			continue
		}
		field := rv.Field(i)
		switch schemaField.Typ {
		case schema.TimeType:
			switch v := rawVal.(type) {
			case time.Time:
				field.Set(reflect.ValueOf(v.UTC()))
			case string:
				// fallback: ISO8601 string
				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return &errors.Error{
						Op:    op,
						Field: tag,
						Err:   fmt.Errorf("%w: invalid time string format", errors.ErrValidation),
					}
				}
				field.Set(reflect.ValueOf(t.UTC()))
			default:
				return &errors.Error{
					Op:    op,
					Field: tag,
					Err:   fmt.Errorf("%w: unsupported type %T for time field", errors.ErrValidation, v),
				}
			}
		case schema.IntegerType:
			// JSON numbers are always float64
			switch v := rawVal.(type) {
			case float64:
				field.Set(reflect.ValueOf(v).Convert(field.Type()))
			default:
				return errors.Errorf(op, fmt.Errorf("%w", errors.ErrValidation), "unexpected type for int field %s", tag)
			}
		default:
			rv2 := reflect.ValueOf(rawVal)
			if !rv2.Type().AssignableTo(field.Type()) {
				return errors.Errorf(op, fmt.Errorf("%w", errors.ErrValidation), "cannot assign value of type %s to field %s of type %s", rv2.Type(), tag, field.Type())
			}
			field.Set(rv2)
		}
	}

	return nil
}
