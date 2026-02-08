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

		if err := bookSchema.Scan(ctx, &books, scanInput); err != nil {
			t.Fatalf("Failed to scan books: %v", err)
		}

		if len(books) != 2 {
			t.Fatalf("Expected at least one book in scan results")
		}

		t.Logf("Successfully scanned %d books (limited to 2)", len(books))
	})

		scanInput := &gardb.ScanInput{
			Limit:     10,
			NextToken: nil,
		}
		if err := bookSchema.Scan(ctx, &books, scanInput); err != nil {
			t.Fatalf("Failed to scan books: %v", err)
		}

		if len(books) == 0 {
			t.Fatalf("Expected at least one book in scan results")
		}

		t.Logf("Successfully scanned books: %+v", books)
	})
}
