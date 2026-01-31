package gardb

import (
	"context"
	"strings"
	"testing"
)

func TestGet_ReturnsValidationError_ForNonPointer(t *testing.T) {
	s := &Schema{}
	err := s.Get(context.Background(), "id", 123)
	if err == nil {
		t.Fatalf("expected validation error for non-pointer obj")
	}
	if !strings.Contains(err.Error(), "expected pointer to struct with GardbMeta field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGet_ReturnsValidationError_ForStructWithoutGardbMeta(t *testing.T) {
	type NoMeta struct {
		Name string
	}
	s := &Schema{}
	err := s.Get(context.Background(), "id", &NoMeta{Name: "x"})
	if err == nil {
		t.Fatalf("expected validation error for struct without GardbMeta")
	}
	if !strings.Contains(err.Error(), "expected pointer to struct with GardbMeta field") {
		t.Fatalf("unexpected error: %v", err)
	}
}
