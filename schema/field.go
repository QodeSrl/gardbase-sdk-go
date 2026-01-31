package schema

import (
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
)

type FieldType int

const (
	StringType FieldType = iota
	IntegerType
	BooleanType
	FloatType
	TimeType
	JSONType
)

type Field struct {
	Name          string
	typ           FieldType
	TypeValidator func(any) bool
	defaultValue  any
	required      bool
	searchable    bool
}

func String() *Field {
	return &Field{
		typ: StringType,
		TypeValidator: func(val any) bool {
			_, ok := val.(string)
			return ok
		},
	}
}

func Int() *Field {
	return &Field{
		typ: IntegerType,
		TypeValidator: func(val any) bool {
			rv := reflect.ValueOf(val)
			return rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Int64
		},
	}
}

func Bool() *Field {
	return &Field{
		typ: BooleanType,
		TypeValidator: func(val any) bool {
			_, ok := val.(bool)
			return ok
		},
	}
}

func Float() *Field {
	return &Field{
		typ: FloatType,
		TypeValidator: func(val any) bool {
			rv := reflect.ValueOf(val)
			return rv.Kind() == reflect.Float32 || rv.Kind() == reflect.Float64
		},
	}
}

func JSON() *Field {
	return &Field{
		typ: JSONType,
		TypeValidator: func(val any) bool {
			_, ok := val.(map[string]any)
			return ok
		},
	}
}

func Time() *Field {
	return &Field{
		typ: TimeType,
		TypeValidator: func(val any) bool {
			_, ok := val.(int64)
			return ok
		},
	}
}

func (f *Field) Default(val any) *Field {
	if !f.TypeValidator(val) {
		panic("default value type does not match field type")
	}
	f.defaultValue = val
	return f
}

func (f *Field) Required() *Field {
	f.required = true
	return f
}

func (f *Field) Searchable() *Field {
	f.searchable = true
	return f
}

func (f *Field) IsRequired() bool {
	return f.required
}

func (f *Field) ExtractIntoValuesIndexes(val reflect.Value, values *map[string]any, indexes *map[string]any, valErrors *errors.ValidationErrors, tag string) {
	if val.IsZero() {
		if f.defaultValue != nil {
			(*values)[tag] = f.defaultValue
		} else if f.required {
			valErrors.Add(tag, "field is required", nil)
			return
		} else {
			return
		}
	} else {
		if !f.TypeValidator(val.Interface()) {
			valErrors.Add(tag, "invalid type", val.Interface())
			return
		}
		(*values)[tag] = val.Interface()
		if f.searchable {
			(*indexes)[tag] = val.Interface()
		}
	}
}
