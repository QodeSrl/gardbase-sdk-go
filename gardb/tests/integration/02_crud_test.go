//go:build integration
// +build integration

package integration

import (
	"testing"
	"time"
)

func TestCRUD_CreateObject(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	t.Run("create_with_all_fields", func(t *testing.T) {
		book := Book{
			Name:        "Test Book",
			Author:      "Test Author",
			Pages:       300,
			PublishedAt: time.Now(),
			ISBN:        "978-1234567890",
			InStock:     true,
		}

		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to create book: %v", err)
		}

		if book.ID == "" {
			t.Fatalf("Expected book ID to be set after creation")
		}
		if book.Version != 1 {
			t.Fatalf("Expected book version to be 1 after creation, got %d", book.Version)
		}
		if book.CreatedAt.IsZero() {
			t.Fatalf("Expected book CreatedAt to be set after creation")
		}
		if book.UpdatedAt.IsZero() {
			t.Fatalf("Expected book UpdatedAt to be set after creation")
		}

		// Cleanup
		bookSchema.Delete(fixture.Ctx, book.ID)
	})

	t.Run("create_multiple_objects_same_values", func(t *testing.T) {
		books := []Book{
			{Name: "Duplicate Book", Author: "Same Author", Pages: 200, PublishedAt: time.Now(), ISBN: "978-1111111111", InStock: true},
			{Name: "Duplicate Book", Author: "Same Author", Pages: 200, PublishedAt: time.Now(), ISBN: "978-1111111111", InStock: true},
			{Name: "Duplicate Book", Author: "Same Author", Pages: 200, PublishedAt: time.Now(), ISBN: "978-1111111111", InStock: true},
		}

		ids := make([]string, len(books))
		for i := range books {
			if err := bookSchema.Put(fixture.Ctx, &books[i]); err != nil {
				t.Fatalf("Failed to create book %d: %v", i+1, err)
			}
			ids[i] = books[i].ID
		}

		// Verify all books were created with unique IDs
		for i, id := range ids {
			retrieved, err := bookSchema.Get(fixture.Ctx, id)
			if err != nil {
				t.Fatalf("Failed to retrieve book %d: %v", i+1, err)
			}
			if retrieved.Name != "Duplicate Book" {
				t.Errorf("Expected book %d name to be 'Duplicate Book', got '%s'", i+1, retrieved.Name)
			}
		}

		// Cleanup
		for _, id := range ids {
			bookSchema.Delete(fixture.Ctx, id)
		}
	})
}

func TestCRUD_ReadObject(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	book := Book{
		Name:        "Read Test Book",
		Author:      "Read Test Author",
		Pages:       150,
		PublishedAt: time.Now(),
		ISBN:        "978-2222222222",
		InStock:     true,
	}
	bookSchema.Put(fixture.Ctx, &book)
	defer bookSchema.Delete(fixture.Ctx, book.ID)

	t.Run("read_existing_object", func(t *testing.T) {
		retrieved, err := bookSchema.Get(fixture.Ctx, book.ID)
		if err != nil {
			t.Fatalf("Failed to read existing book: %v", err)
		}

		AssertBookEqual(t, retrieved, &book)
	})

	t.Run("read_nonexistent_object", func(t *testing.T) {
		tempBook := Book{
			Name:        "Temp Book",
			Author:      "Temp Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "978-3333333333",
			InStock:     true,
		}
		bookSchema.Put(fixture.Ctx, &tempBook)
		bookSchema.Delete(fixture.Ctx, tempBook.ID)

		// try to get
		_, err := bookSchema.Get(fixture.Ctx, tempBook.ID)
		if err == nil {
			t.Fatalf("Expected error when reading deleted book, got nil")
		}
	})
}

func TestCRUD_UpdateObject(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	t.Run("update_via_put", func(t *testing.T) {
		book := Book{
			Name:        "Original",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "978-4444444444",
			InStock:     true,
		}
		bookSchema.Put(fixture.Ctx, &book)
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		originalVersion := book.Version

		// update
		book.Name = "Updated"
		book.Pages = 200
		book.InStock = false

		err := bookSchema.Put(fixture.Ctx, &book)
		if err != nil {
			t.Fatalf("Failed to update book via Put: %v", err)
		}

		// verify version incremented
		if book.Version != originalVersion+1 {
			t.Fatalf("Expected book version to increment by 1 after update, got %d", book.Version)
		}

		retrieved, _ := bookSchema.Get(fixture.Ctx, book.ID)
		if retrieved.Name != "Updated" {
			t.Errorf("Expected book name to be 'Updated', got '%s'", retrieved.Name)
		}
		if retrieved.Pages != 200 {
			t.Errorf("Expected book pages to be 200, got %d", retrieved.Pages)
		}
	})

	t.Run("update_via_update_method", func(t *testing.T) {
		book := Book{
			Name:        "Original",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "978-5555555555",
			InStock:     true,
		}
		bookSchema.Put(fixture.Ctx, &book)
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		updated, err := bookSchema.Update(fixture.Ctx, book.ID, func(b *Book) error {
			b.Name = "Updated via Update"
			b.Pages = 250
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to update book via Update method: %v", err)
		}

		if updated.Version != book.Version+1 {
			t.Fatalf("Expected book version to increment by 1 after update, got %d", updated.Version)
		}
		if updated.Name != "Updated via Update" {
			t.Errorf("Expected book name to be 'Updated via Update', got '%s'", updated.Name)
		}
		if updated.Pages != 250 {
			t.Errorf("Expected book pages to be 250, got %d", updated.Pages)
		}
	})

	t.Run("concurrent_update_conflict", func(t *testing.T) {
		book := Book{
			Name:        "Conflict Book",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "978-6666666666",
			InStock:     true,
		}
		bookSchema.Put(fixture.Ctx, &book)
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		// Simulate two clients with same version
		book1, _ := bookSchema.Get(fixture.Ctx, book.ID)
		book2, _ := bookSchema.Get(fixture.Ctx, book.ID)

		// first update should succeed
		book1.Pages = 200
		err1 := bookSchema.Put(fixture.Ctx, book1)
		if err1 != nil {
			t.Fatalf("First update failed: %v", err1)
		}

		// second update should fail due to version conflict
		book2.Pages = 300
		err2 := bookSchema.Put(fixture.Ctx, book2)
		if err2 == nil {
			t.Fatalf("Expected version conflict error on second update, got nil")
		}
	})
}

func TestCRUD_DeleteObject(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	t.Run("delete_existing_object", func(t *testing.T) {
		book := Book{
			Name:        "Delete Test Book",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "978-7777777777",
			InStock:     true,
		}
		bookSchema.Put(fixture.Ctx, &book)

		err := bookSchema.Delete(fixture.Ctx, book.ID)
		if err != nil {
			t.Fatalf("Failed to delete existing book: %v", err)
		}

		// verify deletion
		_, err = bookSchema.Get(fixture.Ctx, book.ID)
		if err == nil {
			t.Fatalf("Expected error when reading deleted book, got nil")
		}
	})

	t.Run("delete_nonexistent_object", func(t *testing.T) {
		err := bookSchema.Delete(fixture.Ctx, "nonexistent-1234")
		if err == nil {
			t.Fatalf("Expected error when deleting nonexistent book, got nil")
		}
	})

	t.Run("delete_twice", func(t *testing.T) {
		book := Book{
			Name:        "Delete Twice Book",
			Author:      "Author",
			Pages:       100,
			PublishedAt: time.Now(),
			ISBN:        "978-8888888888",
			InStock:     true,
		}
		bookSchema.Put(fixture.Ctx, &book)

		// first delete should succeed
		err := bookSchema.Delete(fixture.Ctx, book.ID)
		if err != nil {
			t.Fatalf("Failed to delete book the first time: %v", err)
		}

		// second delete should return an error
		err = bookSchema.Delete(fixture.Ctx, book.ID)
		if err == nil {
			t.Fatalf("Expected error when deleting the same book twice, got nil")
		}
	})
}
