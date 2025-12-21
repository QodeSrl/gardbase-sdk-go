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

func (s *Schema) New(data map[string]any) *Record {
	result := make([]RecordField, 0, len(s.fields))

	for fieldName, field := range s.fields {
		val := data[fieldName]
		if val == nil && field.required {
			panic("missing required field: " + fieldName)
		}
		if val == nil && field.defaultValue != nil {
			val = field.defaultValue
		}

		if val != nil && !field.typeValidator(val) {
			panic("invalid type for field: " + fieldName)
		}

		result = append(result, RecordField{
			name:       fieldName,
			typ:        field.typ,
			value:      val,
			searchable: field.searchable,
		})
	}
	return &Record{
		schema: s,
		fields: result,
	}
}
