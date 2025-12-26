package schema

import (
	"fmt"
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/google/uuid"
)

type Schema struct {
	name   string
	fields Model
}

type GardbMeta struct {
	schema    *Schema
	id        string
	CreatedAt int64
	UpdatedAt int64
}

func (m *GardbMeta) ID() string {
	return m.id
}

func (m *GardbMeta) Schema() *Schema {
	return m.schema
}

type Model map[string]*Field

func New(name string, model Model) (*Schema, error) {
	const op = "Schema.New"

	fields := make(map[string]*Field, len(model))
	for fieldName, field := range model {
		if field.defaultValue != nil && !field.typeValidator(field.defaultValue) {
			return nil, &errors.Error{
				Op:    op,
				Field: fieldName,
				Err:   fmt.Errorf("%w: default value type does not match field type", errors.ErrInvalidSchema),
			}
		}
		field.name = fieldName
		fields[fieldName] = field
	}
	return &Schema{
		name:   name,
		fields: fields,
	}, nil
}

func (s *Schema) Name() string {
	return s.name
}

func (s *Schema) New(ptr any) error {
	const op = "Schema.New"

	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: expected pointer to struct, got %T", errors.ErrInvalidSchema, ptr),
		}
	}
	rv = rv.Elem()
	if rv.FieldByName("GardbMeta").IsValid() == false {
		return errors.Errorf(op, nil, "struct must have a GardbMeta field of type *GardbMeta")
	}
	if rv.FieldByName("GardbMeta").Type() != reflect.TypeOf((*GardbMeta)(nil)) {
		return errors.Errorf(op, nil, "struct must have a GardbMeta field of type *GardbMeta")
	}

	rv = rv.Elem()
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
		if !val.IsZero() && !sField.typeValidator(val.Interface()) {
			return &errors.Error{
				Op:    op,
				Field: tag,
				Err:   fmt.Errorf("%w: invalid type", errors.ErrInvalidSchema),
			}
		}
	}

	// Check if all schema fields were processed
	for fieldName, field := range s.fields {
		if field.required && !structTags[fieldName] {
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
			id:     uuid.New().String(),
		}
		metaField.Set(reflect.ValueOf(meta))
	}

	return nil
}

func (s *Schema) Extract(ptr any) (values map[string]any, indexes map[string]any, err error) {
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

		field := s.fields[tag]
		val := rv.Field(i)

		if val.IsZero() {
			if field.defaultValue != nil {
				values[tag] = field.defaultValue
			} else if field.required {
				valErrors.Add(tag, "field is required", nil)
				continue
			}
		}

		if !val.IsZero() && !field.typeValidator(val.Interface()) {
			valErrors.Add(tag, "invalid type", val.Interface())
		}

		values[tag] = val.Interface()

		if field.searchable {
			indexes[tag] = val.Interface()
		}
	}

	if valErrors.HasErrors() {
		return nil, nil, valErrors
	}

	return values, indexes, nil
}
