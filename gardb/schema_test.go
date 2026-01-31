package gardb

import "testing"

func TestValidatePtrToStructWithGardbMeta(t *testing.T) {
	type Good struct {
		GardbMeta GardbMeta
	}
	if !validatePtrToStructWithGardbMeta(&Good{}) {
		t.Fatal("expected true for pointer to struct with GardbMeta")
	}

	var n int
	if validatePtrToStructWithGardbMeta(&n) {
		t.Fatal("expected false for pointer to non-struct")
	}

	type NoMeta struct {
		A int
	}
	if validatePtrToStructWithGardbMeta(&NoMeta{}) {
		t.Fatal("expected false for struct without GardbMeta")
	}

	type WrongMeta struct {
		GardbMeta int
	}
	if validatePtrToStructWithGardbMeta(&WrongMeta{}) {
		t.Fatal("expected false for struct with wrong GardbMeta type")
	}

	var g Good
	if validatePtrToStructWithGardbMeta(g) {
		t.Fatal("expected false for non-pointer input")
	}
}

func TestSchemaNewSetsGardbMetaAndErrors(t *testing.T) {
	type ModelWithMeta struct {
		GardbMeta GardbMeta
		A         int
	}
	s := &Schema{name: "test", fields: Model{}}

	m := &ModelWithMeta{}
	if err := s.new("op", m); err != nil {
		t.Fatalf("unexpected error from new: %v", err)
	}
	if m.GardbMeta.Schema() != s {
		t.Fatal("expected GardbMeta.schema to be set to the schema")
	}

	// Missing GardbMeta field should return an error
	type NoMeta struct {
		A int
	}
	n := &NoMeta{}
	if err := s.new("op", n); err == nil {
		t.Fatal("expected error when struct lacks GardbMeta field")
	}
}

func TestSchemaExtract_NoTagsReturnsEmptyMaps(t *testing.T) {
	type ModelNoTags struct {
		GardbMeta GardbMeta
		A         int
	}
	s := &Schema{fields: Model{"name": nil}}
	m := &ModelNoTags{}
	values, indexes, err := s.extract(m)
	if err != nil {
		t.Fatalf("unexpected error from extract: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected empty values map, got %v", values)
	}
	if len(indexes) != 0 {
		t.Fatalf("expected empty indexes map, got %v", indexes)
	}
}
