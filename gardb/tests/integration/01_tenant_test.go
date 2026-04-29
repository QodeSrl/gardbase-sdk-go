//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

func TestTenant_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	fixture1 := Setup(t)
	fixture2 := Setup(t)

	schema1 := fixture1.CreateBookSchema(t)
	schema2 := fixture2.CreateBookSchema(t)

	t.Run("data_isolation", func(t *testing.T) {
		book1 := Book{
			Name:        "Clean Code",
			Author:      "Robert C. Martin",
			Pages:       464,
			PublishedAt: time.Date(2008, 8, 1, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0132350884",
			InStock:     true,
		}
		if err := schema1.Put(ctx, &book1); err != nil {
			t.Fatalf("Failed to create book in tenant 1: %v", err)
		}

		book2 := Book{
			Name:        "The Pragmatic Programmer",
			Author:      "Andrew Hunt",
			Pages:       352,
			PublishedAt: time.Date(1999, 10, 30, 0, 0, 0, 0, time.UTC),
			ISBN:        "978-0201616224",
			InStock:     true,
		}
		if err := schema2.Put(ctx, &book2); err != nil {
			t.Fatalf("Failed to create book in tenant 2: %v", err)
		}

		books1, err := schema1.Scan(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to scan books in tenant 1: %v", err)
		}
		if len(books1.Items) != 1 || books1.Items[0].ID != book1.ID || books1.Count != 1 {
			t.Fatalf("Expected to find only book1 in tenant 1, found: %v", books1)
		}

		books2, err := schema2.Scan(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to scan books in tenant 2: %v", err)
		}
		if len(books2.Items) != 1 || books2.Items[0].ID != book2.ID || books2.Count != 1 {
			t.Fatalf("Expected to find only book2 in tenant 2, found: %v", books2)
		}

		t.Log("Tenant isolation test passed: each tenant can only access its own data")
	})

	t.Run("cross_tenant_access_denied", func(t *testing.T) {
		books2, err := schema2.Scan(ctx, &gardb.ScanInput{Limit: 1})
		if len(books2.Items) == 0 {
			t.Skip("No data in tenant 2 to test cross-tenant access")
		}

		tenant2BookID := books2.Items[0].ID

		book1, err := schema1.Get(ctx, tenant2BookID)
		if err == nil || book1 != nil {
			t.Fatalf("Expected error when tenant 1 tries to access tenant 2's book, but got: %v", book1)
		}

		t.Logf("Cross-tenant access correctly denied: %v", err)
	})

	t.Run("api_key_isolation", func(t *testing.T) {
		badClient, err := gardb.NewClient(&gardb.Config{
			APIEndpoint:         fixture1.APIEndpoint,
			APIKey:              fixture1.APIKey,
			TenantID:            fixture2.TenantID,
			VerifyAttestation:   false,
			SkipPCRVerification: true,
		})
		if err != nil {
			t.Fatalf("Failed to create client with wrong API key: %v", err)
		}

		badSchema, err := gardb.Schema[*Book](ctx, badClient, "books", gardb.Model{"name": schema.String()}, nil)
		if err != nil {
			t.Fatalf("Failed to create schema with wrong API key: %v", err)
		}

		_, err = badSchema.Scan(ctx, &gardb.ScanInput{Limit: 1})
		if err == nil {
			t.Fatalf("Expected error when using wrong API key, but scan succeeded")
		}
		t.Logf("API key isolation correctly enforced: %v", err)
	})
}
