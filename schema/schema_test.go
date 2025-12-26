package schema_test

import (
	"testing"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func TestNewConstructor_DefaultTypeMismatch(t *testing.T) {
	if _, err := schema.New("mismatch", schema.Model{
		"f": schema.Bool().Default(123),
	}); err == nil {
		t.Fatalf("expected error for default type mismatch, got nil")
	}
}

func TestNewConstructor_Success(t *testing.T) {
	model := schema.Model{
		"name": schema.String().Searchable().Default("john"),
		"age":  schema.Int().Required(),
	}

	s, err := schema.New("person", model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name() != "person" {
		t.Fatalf("expected schema name 'person', got %q", s.Name())
	}
}

func TestSchemaNew_InvalidInputs(t *testing.T) {
	model := schema.Model{
		"name": schema.String().Searchable().Default("john"),
	}
	s, _ := schema.New("t", model)

	// Non-pointer
	if err := s.New(123); err == nil {
		t.Fatalf("expected error for non-pointer input, got nil")
	}

	// Struct without GardbMeta field
	type NoMeta struct {
		Name string `gardb:"name"`
	}
	n := NoMeta{}
	if err := s.New(&n); err == nil {
		t.Fatalf("expected error for struct missing GardbMeta, got nil")
	}
}

func TestExtract_ValuesAndValidation(t *testing.T) {
	model := schema.Model{
		"name": schema.String().Searchable().Default("john"),
		"age":  schema.Int().Required(),
	}

	s, err := schema.New("person", model)
	if err != nil {
		t.Fatalf("unexpected error creating schema: %v", err)
	}

	type Person struct {
		GardbMeta *schema.GardbMeta
		Name      string `gardb:"name"`
		Age       int    `gardb:"age"`
	}

	// Successful extraction
	p := Person{Name: "Alice", Age: 30}
	values, indexes, err := s.Extract(&p)
	if err != nil {
		t.Fatalf("unexpected error from Extract: %v", err)
	}
	if values["name"] != "Alice" || values["age"] != 30 {
		t.Fatalf("unexpected values: %v", values)
	}
	if idx, ok := indexes["name"]; !ok || idx != "Alice" {
		t.Fatalf("unexpected indexes: %v", indexes)
	}

	// Missing required field -> expect validation error
	p2 := Person{Name: "Bob", Age: 0}
	_, _, err = s.Extract(&p2)
	if err == nil {
		t.Fatalf("expected validation error for missing required 'age', got nil")
	}
}
