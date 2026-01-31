package gardb

import (
	"fmt"
	"reflect"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

type Schema struct {
	name      string
	tableHash string
	fields    Model
	client    *Client
}

type GardbMeta struct {
	schema    *Schema
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (m *GardbMeta) Schema() *Schema {
	return m.schema
}

func validatePtrToStructWithGardbMeta(obj any) bool {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return false
	}
	rv = rv.Elem()
	field := rv.FieldByName("GardbMeta")
	if !field.IsValid() {
		return false
	}
	if field.Type() != reflect.TypeOf(GardbMeta{}) {
		return false
	}
	return true
}

type Model map[string]*schema.Field // schema.String(), schema.Int(), etc.

// Name returns the name of the schema
func (s *Schema) Name() string {
	return s.name
}

// New validates the given struct against the schema and initializes GardbMeta
func (s *Schema) new(op string, ptr any) error {
	rv := reflect.ValueOf(ptr).Elem()
	rt := rv.Type()

	field := rv.FieldByName("GardbMeta")
	if !field.IsValid() {
		return errors.Errorf(op, nil, "struct must have a GardbMeta field of type GardbMeta")
	}
	if field.Type() != reflect.TypeOf(GardbMeta{}) {
		return errors.Errorf(op, nil, "struct must have a GardbMeta field of type GardbMeta")
	}

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

	// Add GardbMeta to the struct
	metaField := reflect.ValueOf(ptr).Elem().FieldByName("GardbMeta")
	if metaField.IsValid() && metaField.CanSet() {
		meta := GardbMeta{
			schema: s,
		}
		metaField.Set(reflect.ValueOf(meta))
	}

	return nil
}

// Extract extracts the values and indexes from the given struct pointer according to the schema
func (s *Schema) extract(ptr any) (values map[string]any, indexes map[string]any, err error) {
	const op = "Schema.Extract"
	valErrors := &errors.ValidationErrors{Op: op}

	values = make(map[string]any, len(s.fields))
	indexes = make(map[string]any)

	rv := reflect.ValueOf(ptr).Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		tag := sf.Tag.Get("gardb")
		if tag == "" {
			continue
		}
		s.fields[tag].ExtractIntoValuesIndexes(rv.Field(i), &values, &indexes, valErrors, tag)
	}

	if valErrors.HasErrors() {
		return nil, nil, valErrors
	}

	return values, indexes, nil
}
