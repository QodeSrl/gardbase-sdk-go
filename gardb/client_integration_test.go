//go:build integration
// +build integration

package gardb_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
	"github.com/QodeSrl/gardbase/pkg/api/tenants"
)

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func TestIntegration_PutGetWorkflow(t *testing.T) {
	apiEndpoint := getEnv("TEST_GARDB_API_ENDPOINT", "https://api.gardbase.com")

	// Create tenant
	httpClient := &http.Client{}
	payload := map[string]string{}
	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiEndpoint+"/api/tenants/", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to create tenant creation request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create tenant: %v", err)
	}
	defer resp.Body.Close()

	var data tenants.CreateTenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode tenant creation response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Unexpected status code %d when creating tenant: %s", resp.StatusCode, data)
	}

	// Init client
	client, err := gardb.NewClient(&gardb.Config{
		APIEndpoint:         apiEndpoint,
		APIKey:              data.APIKey,
		TenantID:            data.TenantID,
		VerifyAttestation:   false,
		SkipPCRVerification: true,
	})
	if err != nil {
		t.Fatalf("Failed to create Gardb client: %v", err)
	}
	ctx := context.Background()

	type Book struct {
		gardb.GardbMeta
		Name        string    `gardb:"name"`
		Author      string    `gardb:"author"`
		Pages       int       `gardb:"pages"`
		PublishedAt time.Time `gardb:"published_at"`
		ISBN        string    `gardb:"isbn"`
		InStock     bool      `gardb:"in_stock"`
	}

	var bookIds []string

	bookSchema, err := client.Schema(ctx, "book", gardb.Model{
		"name":         schema.String().Required(),
		"author":       schema.String().Required(),
		"pages":        schema.Int().Required(),
		"published_at": schema.Time().Required(),
		"isbn":         schema.String().Required(),
		"in_stock":     schema.Bool().Required(),
	})
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	t.Run("01_health_check", func(t *testing.T) {
		t.Log("Checking API connectivity...")
		resp, err := http.Get(getEnv("TEST_GARDB_API_ENDPOINT", "https://api.gardbase.com") + "/api/health/")
		if err != nil {
			t.Fatalf("API not reachable: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Unexpected status code from health check: %d", resp.StatusCode)
		}
		t.Log("API is healthy and reachable")
	})

	// Create single object
	t.Run("02_create_single_object", func(t *testing.T) {
		t.Log("Creating single book object...")

		book := Book{
			Name:        "The Go Programming Language",
			Author:      "Alan A. A. Donovan",
			Pages:       380,
			PublishedAt: time.Date(2015, 10, 26, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0134190440",
			InStock:     true,
		}

		if err := bookSchema.Put(ctx, &book); err != nil {
			t.Fatalf("Failed to put book: %v", err)
		}

		if book.GardbMeta.ID == "" {
			t.Fatalf("Expected book ID to be set after Put")
		}
		bookIds = append(bookIds, book.GardbMeta.ID)
	})

	t.Run("03_create_multiple_objects", func(t *testing.T) {
		t.Log("Creating multiple book objects...")

		books := []Book{
			{
				Name:        "Clean Code",
				Author:      "Robert C. Martin",
				Pages:       464,
				PublishedAt: time.Date(2008, 8, 1, 0, 0, 0, 0, time.UTC),
				ISBN:        "978-0132350884",
				InStock:     true,
			},
			{
				Name:        "Design Patterns",
				Author:      "Erich Gamma",
				Pages:       395,
				PublishedAt: time.Date(1994, 10, 31, 0, 0, 0, 0, time.UTC),
				ISBN:        "978-0201633610",
				InStock:     false,
			},
			{
				Name:        "Refactoring",
				Author:      "Martin Fowler",
				Pages:       448,
				PublishedAt: time.Date(2018, 11, 20, 0, 0, 0, 0, time.UTC),
				ISBN:        "978-0134757599",
				InStock:     true,
			},
		}

		for i := range books {
			if err := bookSchema.Put(ctx, &books[i]); err != nil {
				t.Fatalf("Failed to put book %d: %v", i, err)
			}
			if books[i].GardbMeta.ID == "" {
				t.Fatalf("Expected book %d ID to be set after Put", i)
			}
			bookIds = append(bookIds, books[i].GardbMeta.ID)
		}
		t.Logf("Successfully created %d books", len(books))
	})

	t.Run("04_get_single_object", func(t *testing.T) {
		t.Log("Getting book object from Gardb...")

		var retrievedBook Book
		if err := bookSchema.Get(ctx, bookIds[0], &retrievedBook); err != nil {
			t.Fatalf("Failed to get book: %v", err)
		}

		// validate object fields
		if retrievedBook.Name != "The Go Programming Language" {
			t.Errorf("Expected name 'The Go Programming Language', got '%s'", retrievedBook.Name)
		}
		if retrievedBook.Author != "Alan A. A. Donovan" {
			t.Errorf("Expected author 'Alan A. A. Donovan', got '%s'", retrievedBook.Author)
		}
		if retrievedBook.Pages != 380 {
			t.Errorf("Expected 380 pages, got %d", retrievedBook.Pages)
		}
		if retrievedBook.ISBN != "978-0134190440" {
			t.Errorf("Expected ISBN '978-0134190440', got '%s'", retrievedBook.ISBN)
		}
		if !retrievedBook.InStock {
			t.Error("Expected book to be in stock")
		}

		// validate metadata
		if retrievedBook.GardbMeta.ID != bookIds[0] {
			t.Errorf("Expected ID %s, got %s", bookIds[0], retrievedBook.GardbMeta.ID)
		}
		if retrievedBook.GardbMeta.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}

		t.Logf("Successfully retrieved book: %+v", retrievedBook)
	})

	// Scan with limit
	t.Run("05_scan_with_limit", func(t *testing.T) {
		t.Log("Scanning book table from Gardb...")

		var books []Book
		scanInput := &gardb.ScanInput{
			Limit:     2,
			NextToken: nil,
		}

		if _, err := bookSchema.Scan(ctx, &books, scanInput); err != nil {
			t.Fatalf("Failed to scan books: %v", err)
		}

		if len(books) != 2 {
			t.Fatalf("Expected at least one book in scan results")
		}

		t.Logf("Successfully scanned %d books (limited to 2)", len(books))
	})

	// Scan all objects with pagination
	t.Run("06_scan_all_with_pagination", func(t *testing.T) {
		t.Log("Scanning all books with pagination...")

		var allBooks []Book
		var nextToken *string

		for {
			var books []Book
			scanInput := &gardb.ScanInput{
				Limit:     2,
				NextToken: nextToken,
			}

			scanOutput, err := bookSchema.Scan(ctx, &books, scanInput)
			if err != nil {
				t.Fatalf("Failed to scan books: %v", err)
			}

			allBooks = append(allBooks, books...)

			if scanOutput.NextToken == nil {
				break
			}
			nextToken = scanOutput.NextToken
		}

		if len(allBooks) != len(bookIds) {
			t.Fatalf("Expected to scan %d books, got %d", len(bookIds), len(allBooks))
		}

		t.Logf("Successfully scanned all books with pagination: %d books", len(allBooks))
	})

	// Scan empty table
	t.Run("07_scan_empty_table", func(t *testing.T) {
		t.Log("Scanning empty table...")

		emptySchema, err := client.Schema(ctx, "empty_table", gardb.Model{
			"field": schema.String().Required(),
		})
		if err != nil {
			t.Fatalf("Failed to create empty table schema: %v", err)
		}

		type EmptyRecord struct {
			gardb.GardbMeta
			Field string `gardb:"field"`
		}

		var results []EmptyRecord
		scanInput := &gardb.ScanInput{
			Limit: 10,
		}

		if _, err := emptySchema.Scan(ctx, &results, scanInput); err != nil {
			t.Fatalf("Failed to scan empty table: %v", err)
		}

		if len(results) != 0 {
			t.Fatalf("Expected 0 records from empty table scan, got %d", len(results))
		}

		t.Log("Successfully scanned empty table with 0 results")
	})

	t.Run("08_update_object", func(t *testing.T) {
		t.Log("Updating book object...")

		var book Book
		if err := bookSchema.Get(ctx, bookIds[0], &book); err != nil {
			t.Fatalf("Failed to get book for update: %v", err)
		}

		book.InStock = false
		book.Pages = 400

		if err := bookSchema.Put(ctx, &book); err != nil {
			t.Fatalf("Failed to update book: %v", err)
		}

		var updatedBook Book
		if err := bookSchema.Get(ctx, bookIds[0], &updatedBook); err != nil {
			t.Fatalf("Failed to get updated book: %v", err)
		}

		if updatedBook.InStock {
			t.Error("Expected book to be out of stock after update")
		}
		if updatedBook.Pages != 400 {
			t.Errorf("Expected 400 pages after update, got %d", updatedBook.Pages)
		}

		t.Logf("Successfully updated book: %+v", updatedBook)
	})

	t.Run("09_delete_object", func(t *testing.T) {
		t.Log("Deleting book object...")

		if err := bookSchema.Delete(ctx, bookIds[0]); err != nil {
			t.Fatalf("Failed to delete book: %v", err)
		}

		var deleted Book
		err := bookSchema.Get(ctx, bookIds[0], &deleted)
		if err == nil {
			t.Fatalf("Expected error when getting deleted book, got none")
		}

		t.Log("Successfully deleted book and verified it cannot be retrieved")
	})

	// Verify deletion via scan
	t.Run("10_verify_deletion_via_scan", func(t *testing.T) {
		t.Log("Verifying deletion via scan...")
		var books []Book
		scanInput := &gardb.ScanInput{
			Limit:     10,
			NextToken: nil,
		}
		if _, err := bookSchema.Scan(ctx, &books, scanInput); err != nil {
			t.Fatalf("Failed to scan books: %v", err)
		}
		for _, b := range books {
			if b.GardbMeta.ID == bookIds[0] {
				t.Fatalf("Deleted book with ID %s still found in scan results", bookIds[0])
			}
		}
		t.Log("Deleted book not found in scan results, deletion verified")
	})

	// Large object
	t.Run("11_large_object", func(t *testing.T) {
		t.Skip("Skipping large object test to avoid long execution time in CI")
	})

	// Concurrent operations
	t.Run("12_concurrent_operations", func(t *testing.T) {
		t.Log("Testing concurrent PUT operations...")

		// Create 10 books concurrently
		const concurrency = 10
		errChan := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				book := Book{
					Name:        "Concurrent Book " + string(rune('A'+idx)),
					Author:      "Test Author",
					Pages:       300,
					PublishedAt: time.Now(),
					ISBN:        "978-0000000000",
					InStock:     true,
				}
				errChan <- bookSchema.Put(ctx, &book)
			}(i)
		}

		// Collect errors
		for i := 0; i < concurrency; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent PUT %d failed: %v", i, err)
			}
		}

		t.Logf("%d concurrent PUTs completed", concurrency)
	})

	// invalid operations
	t.Run("13_error_handling", func(t *testing.T) {
		t.Log("Testing error handling...")

		// Get non-existent object
		var book Book
		err := bookSchema.Get(ctx, "non-existent-id", &book)
		if err == nil {
			t.Error("Expected error when getting non-existent object")
		}
		t.Log("Get non-existent object returned error")

		// Delete non-existent object
		err = bookSchema.Delete(ctx, "non-existent-id")
		if err == nil {
			t.Error("Expected error when deleting non-existent object")
		}
		t.Log("Delete non-existent object returned error")

		// Put with missing required field
		type InvalidBook struct {
			gardb.GardbMeta
			Name string `gardb:"name"` // Missing required 'author' field
		}
		invalidSchema, _ := client.Schema(ctx, "invalid_book", gardb.Model{
			"name":   schema.String().Required(),
			"author": schema.String().Required(),
		})
		invalidBook := InvalidBook{Name: "Test"}
		err = invalidSchema.Put(ctx, &invalidBook)
		if err == nil {
			t.Error("Expected error when putting object with missing required fields")
		}
		t.Log("Put with missing required field returned error")
	})
}
