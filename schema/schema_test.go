package schema_test

import (
	"testing"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func TestSchema_New(t *testing.T) {
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

	err := userSchema.New(&user)
	if err != nil {
		t.Errorf("Failed to create user: %v", err)
	}
}
