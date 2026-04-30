//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
	"time"
)

func TestEdgeCases_DataTypes(t *testing.T) {
	fixture := Setup(t)

	t.Run("empty_string", func(t *testing.T) {
		bookSchema := fixture.CreateBookSchema(t)
		book := Book{
			Name:        "", // Empty string
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "empty-isbn",
			InStock:     true,
		}
		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book with empty string: %v", err)
		}
		defer bookSchema.Delete(fixture.Ctx, book.ID)
		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.Name != "" {
			t.Fatalf("Expected Name to be empty string, got '%s'", retrieved.Name)
		}
	})

	t.Run("zero_values", func(t *testing.T) {
		bookSchema := fixture.CreateBookSchema(t)
		book := Book{
			Name:        "Zero Pages",
			Author:      "Author",
			Pages:       0,           // Zero value
			PublishedAt: time.Time{}, // Zero time
			ISBN:        "000",
			InStock:     false,
		}
		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book with zero values: %v", err)
		}
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.Pages != 0 {
			t.Fatalf("Expected Pages to be 0, got %d", retrieved.Pages)
		}
	})

	t.Run("unicode_content", func(t *testing.T) {
		bookSchema := fixture.CreateBookSchema(t)
		book := Book{
			Name:        "日本語のタイトル 🎌",
			Author:      "作者名",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "unicode-isbn",
			InStock:     true,
		}

		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book with unicode content: %v", err)
		}
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.Name != book.Name {
			t.Fatalf("Expected Name to be '%s', got '%s'", book.Name, retrieved.Name)
		}
	})

	t.Run("very_long_string", func(t *testing.T) {
		bookSchema := fixture.CreateBookSchema(t)
		longString := strings.Repeat("A", 10000)

		book := Book{
			Name:        longString,
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "long-isbn",
			InStock:     true,
		}

		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book with very long string: %v", err)
		}
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.Name != longString {
			t.Fatalf("Expected Name to be long string of length %d, got length %d", len(longString), len(retrieved.Name))
		}
	})
}

func TestEdgeCases_Timestamps(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	t.Run("very_old_date", func(t *testing.T) {
		book := Book{
			Name:        "Old Book",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
			ISBN:        "old-isbn",
			InStock:     true,
		}

		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book with very old date: %v", err)
		}
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.PublishedAt.Year() != 1900 {
			t.Fatalf("Expected PublishedAt year to be 1900, got %d", retrieved.PublishedAt.Year())
		}
	})

	t.Run("future_date", func(t *testing.T) {
		book := Book{
			Name:        "Future Book",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
			ISBN:        "future-isbn",
			InStock:     true,
		}

		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book with future date: %v", err)
		}
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.PublishedAt.Year() != 2100 {
			t.Fatalf("Expected PublishedAt year to be 2100, got %d", retrieved.PublishedAt.Year())
		}
	})
}
