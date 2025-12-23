package schema_test

import (
	"strings"
	"testing"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func TestSchema_New_Success(t *testing.T) {
	type User struct {
		Name string `gardb:"name"`
		Age  int    `gardb:"age"`
		schema.GardbMeta
	}

	userSchema := schema.New("user", schema.Model{
		"name": schema.String().Searchable(),
		"age":  schema.Int().Required(),
	})

	user := User{
		Name: "Alice",
		Age:  30,
	}

	if err := userSchema.New(&user); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if user.ID() == "" {
		t.Fatalf("expected ID to be set")
	}
}

func TestSchema_New_MissingRequired(t *testing.T) {
	type User struct {
		Name string `gardb:"name"`
		Age  int    `gardb:"age"`
	}

	userSchema := schema.New("user", schema.Model{
		"name": schema.String(),
		"age":  schema.Int().Required(),
	})

	user := User{
		Name: "Bob",
		Age:  0,
	}

	err := userSchema.New(&user)
	if err == nil || !strings.Contains(err.Error(), "missing required field: age") {
		t.Fatalf("expected missing required field error for age, got: %v", err)
	}
}

func TestSchema_New_TypeValidation(t *testing.T) {
	type User struct {
		Name string `gardb:"name"`
		Age  int64  `gardb:"age"`
	}

	userSchema := schema.New("user", schema.Model{
		"name": schema.String(),
		"age":  schema.Int(),
	})

	user := User{
		Name: "Carol",
		Age:  25,
	}

	err := userSchema.New(&user)
	if err == nil || !strings.Contains(err.Error(), "field age has invalid type") {
		t.Fatalf("expected invalid type error for age, got: %v", err)
	}
}

func TestSchema_New_DefaultApplied(t *testing.T) {
	type Item struct {
		Count int `gardb:"count"`
	}

	s := schema.New("item", schema.Model{
		"count": schema.Int().Default(42),
	})

	it := Item{}
	if err := s.New(&it); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if it.Count != 42 {
		t.Fatalf("expected default applied (42), got: %d", it.Count)
	}
}

func TestSchema_New_InvalidInput(t *testing.T) {
	type X struct {
		A int `gardb:"a"`
	}

	s := schema.New("x", schema.Model{
		"a": schema.Int(),
	})

	x := X{A: 1}
	// non-pointer
	if err := s.New(x); err == nil || !strings.Contains(err.Error(), "expected pointer to struct") {
		t.Fatalf("expected pointer-to-struct error for non-pointer input, got: %v", err)
	}
	// pointer to non-struct
	n := 5
	if err := s.New(&n); err == nil || !strings.Contains(err.Error(), "expected pointer to struct") {
		t.Fatalf("expected pointer-to-struct error for pointer to non-struct, got: %v", err)
	}
}

func TestSchema_New_UnknownTag(t *testing.T) {
	type Bad struct {
		Other string `gardb:"other"`
	}

	s := schema.New("t", schema.Model{
		"known": schema.String(),
	})

	b := Bad{Other: "x"}
	err := s.New(&b)
	if err == nil || !strings.Contains(err.Error(), "struct field other not defined in schema") {
		t.Fatalf("expected undefined schema field error, got: %v", err)
	}
}
