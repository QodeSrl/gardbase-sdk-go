package gardb_test

import (
	"context"
	"os"
	"testing"

	"github.com/QodeSrl/gardbase-sdk-go/gardb"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func TestIntegration_PutGetWorkflow(t *testing.T) {
	client, err := gardb.NewClient(&gardb.Config{
		APIEndpoint:         getEnv("TEST_GARDB_API_ENDPOINT", "https://api.gardbase.com"),
		KMSKeyID:            getEnv("TEST_GARDB_KMS_KEY_ID", ""),
		VerifyAttestation:   false,
		SkipPCRVerification: true,
	})
	if err != nil {
		t.Fatalf("Failed to create Gardb client: %v", err)
	}
	ctx := context.Background()

	type Book struct {
		schema.GardbMeta
		Name   string `gardb:"name"`
		Author string `gardb:"author"`
		Pages  int    `gardb:"pages"`
	}

	var bookId string

	t.Run("create object", func(t *testing.T) {
		book := Book{
			Name:   "The Go Programming Language",
			Author: "Alan A. A. Donovan",
			Pages:  380,
		}

		if err := client.Put(ctx, &book); err != nil {
			t.Fatalf("Failed to put book: %v", err)
		}

		bookId = book.GardbMeta.ID
	})

	t.Run("retrieve object", func(t *testing.T) {
		var retrievedBook Book

		if err := client.Get(ctx, bookId, &retrievedBook); err != nil {
			t.Fatalf("Failed to get book: %v", err)
		}

		if retrievedBook.Name != "The Go Programming Language" ||
			retrievedBook.Author != "Alan A. A. Donovan" ||
			retrievedBook.Pages != 380 {
			t.Fatalf("Retrieved book does not match expected values")
		}
	})
}
