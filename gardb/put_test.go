package gardb

import (
	"context"
	"strings"
	"testing"
)

func TestPut_ContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &Client{}
	err := c.Put(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "context error") {
		t.Fatalf("expected context error, got: %v", err)
	}
}

func TestPut_InvalidPointer(t *testing.T) {
	c := &Client{}
	obj := &struct{ X int }{X: 1}
	err := c.Put(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "expected pointer to struct with GardbMeta field") {
		t.Fatalf("expected validation error about GardbMeta field, got: %v", err)
	}
}

func TestPut_WrongGardbMetaType(t *testing.T) {
	c := &Client{}
	type S struct {
		GardbMeta int
	}
	obj := &S{GardbMeta: 1}
	err := c.Put(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "GardbMeta field has wrong type") {
		t.Fatalf("expected GardbMeta type error, got: %v", err)
	}
}
