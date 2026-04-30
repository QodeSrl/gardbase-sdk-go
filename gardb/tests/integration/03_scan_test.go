//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qodesrl/gardbase-sdk-go/gardb"
	"github.com/qodesrl/gardbase-sdk-go/schema"
)

func TestScan_Pagination(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	const totalBooks = 25
	createdIDs := make([]string, totalBooks)

	t.Run("setup_test_data", func(t *testing.T) {
		for i := 0; i < totalBooks; i++ {
			book := Book{
				Name:        "Book " + fmt.Sprint(i+1),
				Author:      "Author " + fmt.Sprint(i+1),
				Pages:       100 + i,
				PublishedAt: time.Now(),
				ISBN:        "978-000000000" + fmt.Sprint(i+1),
				InStock:     true,
			}
			if err := bookSchema.Put(fixture.Ctx, &book); err != nil {
				t.Fatalf("Failed to create book %d: %v", i+1, err)
			}
			createdIDs[i] = book.ID
		}
	})

	t.Run("paginate_through_all", func(t *testing.T) {
		const pageSize = 10
		allBooks := make([]*Book, 0, totalBooks)
		var nextCursor *string

		pageCount := 0
		for {
			pageCount++

			out, err := bookSchema.Scan(fixture.Ctx, &gardb.ScanInput{
				Limit:  pageSize,
				Cursor: nextCursor,
			})
			if err != nil {
				t.Fatalf("Failed to scan page %d: %v", pageCount, err)
			}

			allBooks = append(allBooks, out.Items...)
			if out.NextCursor == nil {
				break
			}
			nextCursor = out.NextCursor

			if pageCount > 5 {
				t.Fatalf("Too many pages scanned, possible infinite loop")
			}
		}

		if len(allBooks) != totalBooks {
			t.Fatalf("Expected to retrieve %d books, got %d", totalBooks, len(allBooks))
		}

		t.Logf("Successfully paginated through all %d books in %d pages", totalBooks, pageCount)
	})

	t.Run("consistent_pagination", func(t *testing.T) {
		scan1 := scanAll(t, fixture.Ctx, bookSchema)
		scan2 := scanAll(t, fixture.Ctx, bookSchema)

		if len(scan1) != len(scan2) {
			t.Fatalf("Expected both scans to return the same number of items, got %d and %d", len(scan1), len(scan2))
		}

		ids1 := make(map[string]bool)
		for _, book := range scan1 {
			ids1[book.ID] = true
		}

		for _, book := range scan2 {
			if !ids1[book.ID] {
				t.Fatalf("Book ID %s found in second scan but not in first scan", book.ID)
			}
		}

		t.Logf("Consistent pagination verified with %d items", len(scan1))
	})

	t.Run("empty_scan", func(t *testing.T) {
		emptySchema, err := gardb.Schema[*Book](fixture.Ctx, fixture.Client, "empty_books", gardb.Model{"name": schema.String().Required()}, nil)
		if err != nil {
			t.Fatalf("Failed to create empty schema: %v", err)
		}
		out, err := emptySchema.Scan(fixture.Ctx, &gardb.ScanInput{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("Failed to scan empty schema: %v", err)
		}
		if len(out.Items) != 0 {
			t.Fatalf("Expected to retrieve 0 books from empty schema, got %d", len(out.Items))
		}
		t.Logf("Empty scan returned 0 items as expected")
	})
}

func scanAll[T gardb.GardbObject](t *testing.T, ctx context.Context, s *gardb.GardbSchema[T]) []T {
	var all []T
	var cursor *string

	for {
		out, err := s.Scan(ctx, &gardb.ScanInput{
			Limit:  10,
			Cursor: cursor,
		})
		if err != nil {
			t.Fatalf("Failed to scan: %v", err)
		}

		all = append(all, out.Items...)
		if out.NextCursor == nil {
			break
		}
		cursor = out.NextCursor
	}

	return all
}
