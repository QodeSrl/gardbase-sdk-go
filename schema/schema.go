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
	values    map[string]any
	id        string
	CreatedAt int64
	UpdatedAt int64
}

func (m *GardbMeta) ID() string {
	return m.id
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

func (s *Schema) New(ptrToStruct any) error {
	// Validate that ptrToStruct is a pointer to a struct
	if reflect.TypeOf(ptrToStruct).Kind() != reflect.Ptr || reflect.TypeOf(ptrToStruct).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("expected pointer to struct")
	}

	// Validate that struct fields match schema fields by comparing lengths
	if reflect.ValueOf(ptrToStruct).Elem().NumField() != len(s.fields) {
		return fmt.Errorf("struct does not match schema fields")
	}

	rv := reflect.ValueOf(ptrToStruct).Elem()
	rt := rv.Type()

	values := make(map[string]any, len(s.fields))

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

		// Apply defaults
		if val.IsZero() && sField.defaultValue != nil {
			val.Set(reflect.ValueOf(sField.defaultValue))
		}
		// Check required and type
		if val.IsZero() && sField.required {
			return fmt.Errorf("missing required field: %s", tag)
		}
		// Type validation
		if !val.IsZero() && !sField.typeValidator(val.Interface()) {
			return fmt.Errorf("field %s has invalid type", tag)
		}

		values[tag] = val.Interface()
	}

	// Check if all schema fields were processed
	for name := range s.fields {
		if _, ok := values[name]; !ok {
			return fmt.Errorf("struct does not match schema fields, missing field: %s", name)
		}
	}

	// Add GardbMeta to the struct
	metaField := reflect.ValueOf(ptrToStruct).Elem().FieldByName("GardbMeta")
	if metaField.IsValid() && metaField.CanSet() {
		meta := &GardbMeta{
			schema: s,
			id:     uuid.New().String(),
			values: values,
		}
		metaField.Set(reflect.ValueOf(meta))
	}

	return nil
}
