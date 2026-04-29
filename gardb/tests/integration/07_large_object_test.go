package integration

import (
	"math/rand"
	"testing"

	"github.com/QodeSrl/gardbase-sdk-go/gardb"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func TestLargeObjects_Lifecycle(t *testing.T) {
	fixture := Setup(t)

	type LargeDoc struct {
		gardb.GardbBase
		Title   string `gardb:"title"`
		Content string `gardb:"content"` // Will be > 100KB
	}

	docSchema, _ := gardb.Schema[*LargeDoc](fixture.Ctx, fixture.Client, "documents",
		gardb.Model{
			"title":   schema.String().Required(),
			"content": schema.String().Required(),
		},
		nil,
	)

	t.Run("create_large_object", func(t *testing.T) {
		largeContent := generateLargeString(150 * 1024)

		doc := &LargeDoc{
			Title:   "Large Document",
			Content: largeContent,
		}

		if err := docSchema.Put(fixture.Ctx, doc); err != nil {
			t.Fatalf("Failed to create large document: %v", err)
		}

		retrieved, err := docSchema.Get(fixture.Ctx, doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve large document: %v", err)
		}
		if retrieved.Content != largeContent {
			t.Fatalf("Content mismatch: expected %d chars, got %d chars", len(largeContent), len(retrieved.Content))
		}

		t.Logf("Successfully created and retrieved large document with content size: %d bytes", len(largeContent))
	})

	t.Run("update_small_to_large_object", func(t *testing.T) {
		doc := &LargeDoc{
			Title:   "Small Document",
			Content: "This is a small document.",
		}

		if err := docSchema.Put(fixture.Ctx, doc); err != nil {
			t.Fatalf("Failed to create small document: %v", err)
		}

		largeContent := generateLargeString(150 * 1024)
		doc.Content = largeContent
		if err := docSchema.Put(fixture.Ctx, doc); err != nil {
			t.Fatalf("Failed to update document to large content: %v", err)
		}

		retrieved, err := docSchema.Get(fixture.Ctx, doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated document: %v", err)
		}

		if retrieved.Content != largeContent {
			t.Fatalf("Content mismatch after update: expected %d chars, got %d chars", len(largeContent), len(retrieved.Content))
		}

		t.Logf("Successfully updated document to large content with size: %d bytes", len(largeContent))
	})

	t.Run("update_large_to_small_object", func(t *testing.T) {
		largeContent := generateLargeString(150 * 1024)
		doc := &LargeDoc{
			Title:   "Large Document",
			Content: largeContent,
		}

		if err := docSchema.Put(fixture.Ctx, doc); err != nil {
			t.Fatalf("Failed to create large document: %v", err)
		}

		doc.Content = "This is now a small document."
		if err := docSchema.Put(fixture.Ctx, doc); err != nil {
			t.Fatalf("Failed to update large document to small content: %v", err)
		}

		retrieved, err := docSchema.Get(fixture.Ctx, doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated document: %v", err)
		}
		if retrieved.Content != "This is now a small document." {
			t.Fatalf("Content mismatch after update: expected small content, got %d chars", len(retrieved.Content))
		}

		t.Logf("Successfully updated large document to small content with size: %d bytes", len(retrieved.Content))
	})
}

func generateLargeString(size int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, size)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}
