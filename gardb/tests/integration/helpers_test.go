//go:build integration
// +build integration

package integration

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

type Book struct {
	gardb.GardbBase
	Name        string    `gardb:"name"`
	Author      string    `gardb:"author"`
	Pages       int       `gardb:"pages"`
	PublishedAt time.Time `gardb:"publishedAt"`
	ISBN        string    `gardb:"isbn"`
	InStock     bool      `gardb:"inStock"`
}

type LargeDocument struct {
	gardb.GardbBase
	Title   string `gardb:"title"`
	Content string `gardb:"content"` // large field
}

type User struct {
	gardb.GardbBase
	Email     string    `gardb:"email"`
	Username  string    `gardb:"username"`
	Age       int       `gardb:"age"`
	Active    bool      `gardb:"active"`
	CreatedAt time.Time `gardb:"createdAt"`
}

type TestFixture struct {
	Client      *gardb.Client
	TenantID    string
	APIKey      string
	APIEndpoint string
	Ctx         context.Context
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// Setup creates a new tenant and client for testing
func Setup(t *testing.T) *TestFixture {
	t.Helper()

	apiEndpoint := getEnv("TEST_GARDB_API_ENDPOINT", "http://localhost:8080")

	// health check
	resp, err := http.Get(apiEndpoint + "/api/health/")
	if err != nil {
		t.Fatalf("failed to connect to API endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API endpoint health check failed with status: %s", resp.Status)
	}

	// create tenant
	httpClient := &http.Client{Timeout: 10 * time.Second}
	payload := map[string]string{}
	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiEndpoint+"/api/tenants/", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("failed to create tenant request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}
	defer resp.Body.Close()

	var data tenants.CreateTenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode tenant creation response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tenant creation failed with status: %s", resp.Status)
	}

	// init client
	client, err := gardb.NewClient(&gardb.Config{
		APIEndpoint:         apiEndpoint,
		APIKey:              data.APIKey,
		TenantID:            data.TenantID,
		VerifyAttestation:   false,
		SkipPCRVerification: true,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return &TestFixture{
		Client:      client,
		TenantID:    data.TenantID,
		APIKey:      data.APIKey,
		APIEndpoint: apiEndpoint,
		Ctx:         context.Background(),
	}
}

// CreateBookSchema creates the "books" schema with appropriate indexes for testing
func (f *TestFixture) CreateBookSchema(t *testing.T) *gardb.GardbSchema[*Book] {
	t.Helper()

	schema, err := gardb.Schema[*Book](f.Ctx, f.Client, "books",
		gardb.Model{
			"name":         schema.String().Required(),
			"author":       schema.String().Required(),
			"pages":        schema.Int().Required(),
			"published_at": schema.Time().Required(),
			"isbn":         schema.String().Required(),
			"in_stock":     schema.Bool().Required(),
		},
		gardb.Indexes{
			gardb.Index(gardb.Hash("name"), nil),
			gardb.Index(gardb.Hash("author"), gardb.Range("published_at")),
			gardb.Index(gardb.Hash("isbn"), nil),
		},
	)
	if err != nil {
		t.Fatalf("failed to create book schema: %v", err)
	}
	return schema
}

func SampleBooks() []Book {
	return []Book{
		{
			Name:        "Clean Code",
			Author:      "Robert C. Martin",
			Pages:       464,
			PublishedAt: time.Date(2008, 8, 1, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0132350884",
			InStock:     true,
		},
		{
			Name:        "The Pragmatic Programmer",
			Author:      "Andrew Hunt",
			Pages:       352,
			PublishedAt: time.Date(1999, 10, 30, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0201616224",
			InStock:     true,
		},
		{
			Name:        "Refactoring",
			Author:      "Martin Fowler",
			Pages:       448,
			PublishedAt: time.Date(2018, 11, 20, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0134757599",
			InStock:     true,
		},
		{
			Name:        "Domain-Driven Design",
			Author:      "Eric Evans",
			Pages:       560,
			PublishedAt: time.Date(2003, 8, 22, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0321125217",
			InStock:     false,
		},
	}
}

func AssertBookEqual(t *testing.T, expected, actual *Book) {
	t.Helper()

	if actual.Name != expected.Name {
		t.Errorf("expected Name %s, got %s", expected.Name, actual.Name)
	}
	if actual.Author != expected.Author {
		t.Errorf("expected Author %s, got %s", expected.Author, actual.Author)
	}
	if actual.Pages != expected.Pages {
		t.Errorf("expected Pages %d, got %d", expected.Pages, actual.Pages)
	}
	if !actual.PublishedAt.Equal(expected.PublishedAt) {
		t.Errorf("expected PublishedAt %s, got %s", expected.PublishedAt, actual.PublishedAt)
	}
	if actual.ISBN != expected.ISBN {
		t.Errorf("expected ISBN %s, got %s", expected.ISBN, actual.ISBN)
	}
	if actual.InStock != expected.InStock {
		t.Errorf("expected InStock %v, got %v", expected.InStock, actual.InStock)
	}
}
