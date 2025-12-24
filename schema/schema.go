package schema

import (
	"fmt"
	"reflect"

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

func New(name string, model Model) *Schema {
	fields := make(map[string]*Field, len(model))
	for fieldName, field := range model {
		field.name = fieldName
		fields[fieldName] = field
	}
	return &Schema{
		name:   name,
		fields: fields,
	}
}

func (s *Schema) Name() string {
	return s.name
}

func (s *Schema) New(ptr any) error {
	rv := reflect.ValueOf(ptr)
	// Validate that ptr is a pointer to a struct
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("expected pointer to struct")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		tag := sf.Tag.Get("gardb")
		if tag == "" {
			continue
		}

		// Validate field against schema
		sField, ok := s.fields[tag]
		if !ok {
			return fmt.Errorf("struct field %s not defined in schema", tag)
		}

		val := rv.Field(i)
		if !val.IsZero() && !sField.typeValidator(val.Interface()) {
			return fmt.Errorf("field %s has invalid type", tag)
		}
	}

	// Check if all schema fields were processed
	for name := range s.fields {
		if _, ok := rt.FieldByName(name); !ok {
			return fmt.Errorf("struct does not match schema fields, missing field: %s", name)
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
				return nil, nil, fmt.Errorf("missing required field: %s", tag)
			}
		}

		if !val.IsZero() && !field.typeValidator(val.Interface()) {
			return nil, nil, fmt.Errorf("field %s has invalid type", tag)
		}

		values[tag] = val.Interface()

		if field.searchable {
			indexes[tag] = val.Interface()
		}
	}

	return values, indexes, nil
}
