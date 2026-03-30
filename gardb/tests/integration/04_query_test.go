//go:build integration
// +build integration

package integration

import (
	"testing"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb"
)

func TestQuery_HashIndex(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	books := []Book{
		{Name: "Clean Code", Author: "Robert C. Martin", Pages: 464, PublishedAt: time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC), ISBN: "111", InStock: true},
		{Name: "Clean Code", Author: "Different Author", Pages: 300, PublishedAt: time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC), ISBN: "222", InStock: true}, // Same name
		{Name: "Refactoring", Author: "Martin Fowler", Pages: 448, PublishedAt: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), ISBN: "333", InStock: true},
	}

	ids := make([]string, len(books))
	for i := range books {
		bookSchema.Put(fixture.Ctx, &books[i])
		ids[i] = books[i].ID
	}
	defer func() {
		for _, id := range ids {
			bookSchema.Delete(fixture.Ctx, id)
		}
	}()

	t.Run("query_single_result", func(t *testing.T) {
		result, err := bookSchema.Query(fixture.Ctx).Where("name", gardb.Eq("Refactoring")).Execute()

		if err != nil {
			t.Fatalf("Failed to execute query: %v", err)
		}
		if result.Count != 1 {
			t.Fatalf("Expected 1 result, got %d", result.Count)
		}
		if result.Items[0].Author != "Martin Fowler" {
			t.Fatalf("Expected author 'Martin Fowler', got '%s'", result.Items[0].Author)
		}
	})

	t.Run("query_multiple_results_same_value", func(t *testing.T) {
		result, err := bookSchema.Query(fixture.Ctx).Where("name", gardb.Eq("Clean Code")).Execute()

		if err != nil {
			t.Fatalf("Failed to execute query: %v", err)
		}
		if result.Count != 2 {
			t.Fatalf("Expected 2 results, got %d", result.Count)
		}

		// verify both books are returned
		names := map[string]bool{}
		for _, book := range result.Items {
			names[book.Author] = true
		}
		if !names["Robert C. Martin"] || !names["Different Author"] {
			t.Error("Expected both authors in results")
		}
	})

	t.Run("query_no_results", func(t *testing.T) {
		result, err := bookSchema.Query(fixture.Ctx).Where("name", gardb.Eq("nonexistent")).Execute()

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if result.Count != 0 {
			t.Errorf("Expected 0 results, got %d", result.Count)
		}
	})

	t.Run("query_by_isbn", func(t *testing.T) {
		result, err := bookSchema.Query(fixture.Ctx).Where("isbn", gardb.Eq("111")).Execute()

		if err != nil {
			t.Fatalf("Failed to execute query: %v", err)
		}
		if result.Count != 1 {
			t.Fatalf("Expected 1 result, got %d", result.Count)
		}
	})
}

