package internal

import (
	"reflect"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func ValidatePtrToStructWithGardbMeta(obj any) bool {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return false
	}
	rv = rv.Elem()
	if rv.FieldByName("GardbMeta").IsValid() == false {
		return false
	}
	if rv.FieldByName("GardbMeta").Type() != reflect.TypeOf((*schema.GardbMeta)(nil)) {
		return false
	}
	return true
}
