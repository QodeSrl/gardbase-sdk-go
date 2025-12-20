package schema

type Schema struct {
	name   string
	fields map[string]*Field
}

type RecordField struct {
	name       string
	typ        FieldType
	value      any
	searchable bool
}

type Record struct {
	schema *Schema
	fields []RecordField
}

func New(name string) *Schema {
	return &Schema{
		name:   name,
		fields: make(map[string]*Field),
	}
}

func (s *Schema) Field(name string, f *Field) *Schema {
	f.name = name
	s.fields[name] = f
	return s
}

func (s *Schema) Fields() map[string]*Field {
	return s.fields
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
