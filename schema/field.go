package schema

import (
	"reflect"
	"time"

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
	Typ           FieldType
	TypeValidator func(any) bool
	defaultValue  any
	required      bool
}

func String() *Field {
	return &Field{
		Typ: StringType,
		TypeValidator: func(val any) bool {
			_, ok := val.(string)
			return ok
		},
	}
}

func Int() *Field {
	return &Field{
		Typ: IntegerType,
		TypeValidator: func(val any) bool {
			rv := reflect.ValueOf(val)
			return rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Int64
		},
	}
}

func Bool() *Field {
	return &Field{
		Typ: BooleanType,
		TypeValidator: func(val any) bool {
			_, ok := val.(bool)
			return ok
		},
	}
}

func Float() *Field {
	return &Field{
		Typ: FloatType,
		TypeValidator: func(val any) bool {
			rv := reflect.ValueOf(val)
			return rv.Kind() == reflect.Float32 || rv.Kind() == reflect.Float64
		},
	}
}

func JSON() *Field {
	return &Field{
		Typ: JSONType,
		TypeValidator: func(val any) bool {
			_, ok := val.(map[string]any)
			return ok
		},
	}
}

func Time() *Field {
	return &Field{
		Typ: TimeType,
		TypeValidator: func(val any) bool {
			_, ok := val.(time.Time)
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

func (f *Field) IsRequired() bool {
	return f.required
}

func (f *Field) ExtractIntoValues(val reflect.Value, values *map[string]any, valErrors *errors.ValidationErrors, tag string) {
	isEmpty := false
	switch val.Kind() {
	case reflect.Bool:
		isEmpty = false
	case reflect.Ptr, reflect.Interface:
		isEmpty = val.IsNil()
	default:
		isEmpty = val.IsZero()
	}

	if isEmpty {
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
		if f.Typ == TimeType {
			t := val.Interface().(time.Time)
			unix := t.Unix()
			(*values)[tag] = unix
		} else {
			(*values)[tag] = val.Interface()
		}
	}
}
