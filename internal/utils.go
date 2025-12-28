package internal

import (
	"context"
	"errors"
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func ValidatePtrToStructWithGardbMeta(obj any) bool {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return false
	}
	rv = rv.Elem()
	field := rv.FieldByName("GardbMeta")
	if !field.IsValid() {
		return false
	}
	if field.Type() != reflect.TypeOf(schema.GardbMeta{}) {
		return false
	}
	return true
}

func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
