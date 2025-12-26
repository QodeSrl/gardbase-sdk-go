package schema

import (
	"reflect"
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
	name          string
	typ           FieldType
	typeValidator func(any) bool
	defaultValue  any
	required      bool
	searchable    bool
}

func String() *Field {
	return &Field{
		typ: StringType,
		typeValidator: func(val any) bool {
			_, ok := val.(string)
			return ok
		},
	}
}

func Int() *Field {
	return &Field{
		typ: IntegerType,
		typeValidator: func(val any) bool {
			rv := reflect.ValueOf(val)
			return rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Int64
		},
	}
}

func Bool() *Field {
	return &Field{
		typ: BooleanType,
		typeValidator: func(val any) bool {
			_, ok := val.(bool)
			return ok
		},
	}
}

func Float() *Field {
	return &Field{
		typ: FloatType,
		typeValidator: func(val any) bool {
			rv := reflect.ValueOf(val)
			return rv.Kind() == reflect.Float32 || rv.Kind() == reflect.Float64
		},
	}
}

func JSON() *Field {
	return &Field{
		typ: JSONType,
		typeValidator: func(val any) bool {
			_, ok := val.(map[string]any)
			return ok
		},
	}
}

func Time() *Field {
	return &Field{
		typ: TimeType,
		typeValidator: func(val any) bool {
			_, ok := val.(int64)
			return ok
		},
	}
}

func (f *Field) Default(val any) *Field {
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
