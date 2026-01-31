package gardb

import (
	"context"
	"strings"
	"testing"
)

func TestPut_InvalidObject_NonPointer(t *testing.T) {
	s := &Schema{}
	// non-pointer (struct) should return invalid schema error
	err := s.Put(context.Background(), struct{}{})
	if err == nil {
		t.Fatal("expected error for non-pointer input, got nil")
	}
	if !strings.Contains(err.Error(), "expected pointer to struct") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPut_InvalidObject_PointerToNonStruct(t *testing.T) {
	s := &Schema{}
	i := 1
	// pointer to non-struct should return invalid schema error
	err := s.Put(context.Background(), &i)
	if err == nil {
		t.Fatal("expected error for pointer to non-struct input, got nil")
	}
	if !strings.Contains(err.Error(), "expected pointer to struct") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPut_ContextCancelled(t *testing.T) {
	s := &Schema{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // make ctx.Err() non-nil
	obj := &struct {
		GardbMeta GardbMeta
	}{}
	err := s.Put(ctx, obj)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context error") {
		t.Fatalf("unexpected error: %v", err)
	}
}
