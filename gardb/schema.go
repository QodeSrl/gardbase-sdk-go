package gardb

import (
	"fmt"
	"reflect"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

type Schema struct {
	name       string
	tableHash  string
	fields     Model
	timeFields []string
	client     *Client
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

func validatePtrToSliceOfStructsWithGardbMeta(obj any) bool {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Slice {
		return false
	}
	rv = rv.Elem()
	elemType := rv.Type().Elem()
	if elemType.Kind() != reflect.Struct {
		return false
	}
	field, ok := elemType.FieldByName("GardbMeta")
	if !ok || field.Type != reflect.TypeOf(GardbMeta{}) {
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

func (s *Schema) populate(ptr any, raw map[string]any) error {
	const op = "Schema.populate"
	rv := reflect.ValueOf(ptr).Elem()
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
			case float64:
				field.Set(reflect.ValueOf(time.Unix(int64(v), 0).UTC()))
			default:
				return &errors.Error{Op: op, Field: tag, Err: fmt.Errorf("%w: expected time field to be a number (Unix timestamp)", errors.ErrValidation)}
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
