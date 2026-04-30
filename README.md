<br>

# Gardbase Go SDK

Gardbase is a fully encrypted NoSQL DBaaS (Database-as-a-Service) built on AWS infrastructure that provides true zero-trust security. All data is encrypted client-side before leaving your application, while searchable encryption enables secure server-side indexing and queries. AWS Nitro Enclaves manage encryption keys in hardware-isolated environments, ensuring the backend never sees plaintext data. Think MongoDB Atlas meets end-to-end encryption! Ideal for healthcare, finance, and any application requiring verifiable data confidentiality.

## Features

- 🔒 Zero-Trust Encryption - All data encrypted client-side before transmission
- 🔐 End-to-End Encryption - Server never sees plaintext data
- 🛡️ AWS Nitro Enclaves - Cryptographic operations in isolated, attested environment
- 📊 DynamoDB + S3 Storage - Scalable hybrid storage (inline for small objects, S3 for large)
- 🔍 Encrypted Indexes - Search encrypted data using deterministic encryption
- 🔄 Optimistic Locking - Version-based concurrency control prevents conflicts
- 🚀 Type-Safe Generics - Fully typed SDK with Go 1.18+ generics
- 📖 ORM-like API - Intuitive API inspired by Mongoose and GORM

## Installation

```bash
go get github.com/qodesrl/gardbase-sdk-go
```

## Quick Start

### 1. Create a Gardbase Account

At the moment, you can create a tenant through the Gardbase API itself. Use the `CreateTenant` endpoint to set up your tenant and receive your unique Tenant ID and an API Key. (curl a POST request to /api/tenants/)

```bash
curl -X POST https://api.gardbase.com/api/tenants/ -H "Content-Type: application/json"
```

### 2. Initialize the Client

```go
package main

import (
    "context"
    "log"

    "github.com/qodesrl/gardbase-sdk-go/gardb"
)

func main() {
    ctx := context.Background()

    // Initialize client
    client, err := gardb.NewClient(&gardb.Config{
        APIEndpoint: "https://api.gardbase.com",
        APIKey:      "gdb_live_your_api_key_here",
        TenantID:    "tenant_your_tenant_id",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Client is ready to use!
}
```

### 3. Define a Schema

```go
// Define your data structure
type User struct {
    gardb.GardbBase                    // Embeds ID, Version, timestamps
    Email    string `gardb:"email"`
    Name     string `gardb:"name"`
    Age      int    `gardb:"age"`
    IsActive bool   `gardb:"is_active"`
}

// Create schema with validation and indexes
userSchema, err := gardb.Schema[*User](ctx, client, "users",
    gardb.Model{
        "email":     schema.String().Required(),
        "name":      schema.String().Required(),
        "age":       schema.Int().Required(),
        "is_active": schema.Bool().Required(),
    },
    gardb.Indexes{
        gardb.Index(gardb.Hash("email"), nil),  // Simple index on email
    },
)
if err != nil {
    log.Fatal(err)
}
```

### 4. Create and Query Data

```go
// Create a user
user := &User{
    Email:    "alice@example.com",
    Name:     "Alice Smith",
    Age:      28,
    IsActive: true,
}

err = userSchema.Put(ctx, user)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Created user with ID: %s\n", user.ID)

// Query by indexed field
out, err := userSchema.Query(ctx).
    WhereHash("email", gardb.Eq("alice@example.com")).
    Execute()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found user: %+v\n", out.Items[0])
```

## Core Concepts

### Zero-Trust Architecture

Gardbase implements a true zero-trust model:

1. Client-Side Encryption - All data is encrypted in your application before being sent to Gardbase
2. Envelope Encryption - Each object is encrypted with a unique Data Encryption Key (DEK)
3. AWS Nitro Enclaves - DEKs are only decrypted inside hardware-isolated enclaves
4. Key Isolation - Gardbase operators never have access to your encryption keys or plaintext data

```
Your App → [Encrypt Data] → Gardbase Server → [Encrypted Storage]
              ↓                      ↓
         Client Keys          Enclave Only
```

### GardbBase Struct

All your data structures must embed `gardb.GardbBase` to include necessary metadata fields:

```go
type GardbBase struct {
    GardbMeta
}

type GardbMeta struct {
	ID        string
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

Why embed?

- Automatic ID generation
- Built-in versioning for safe updates
- Timestamp tracking

### Schemas and Indexes

Schemas define:

- Structure - Field names and types
- Validation - Required fields, constraints
- Indexes - Which fields are searchable

```go
schema, err := gardb.Schema[*YourType](ctx, client, "table_name",
    gardb.Model{
        "field1": schema.String().Required(),
        "field2": schema.Int().Min(0).Max(100),
    },
    gardb.Indexes{
        gardb.Index(gardb.Hash("field1"), nil),
    },
)
```

### Hybrid Storage

Gardbase automatically chooses the optimal storage:

- Objects < 100KB → Stored inline in DynamoDB (fast, single request)
- Objects ≥ 100KB → Stored in S3 (scalable, cost-effective)

This is transparent to you - the SDK handles it automatically.

## API Reference

### Client Initialization

```go
gardb.NewClient(config *gardb.Config) (*gardb.Client, error)
```

Creates a new Gardbase client instance.

Parameters:

- `config.APIEndpoint` (string): Base URL of the Gardbase API (usually https://api.gardbase.com)
- `config.APIKey` (string): Your API key for authentication
- `config.TenantID` (string): Your tenant ID
- `config.VerifyAttestation` (bool, optional): Whether to verify enclave attestation (default: true)
- `config.SkipPCRVerification` (bool, optional): Skip PCR verification for testing (default: false)

Example:

```go
client, err := gardb.NewClient(&gardb.Config{
    APIEndpoint: "https://api.gardbase.com",
    APIKey:      os.Getenv("GARDBASE_API_KEY"),
    TenantID:    os.Getenv("GARDBASE_TENANT_ID"),
})
```

### Schema Definition

```go
gardb.Schema[T](ctx, client, tableName, model, indexes) (*GardbSchema[T], error)
```

Defines a schema for a specific data type.

Type Parameters:

- `T`: Your custom struct type that embeds `GardbBase`

Parameters:

- `tableName` (string): Name of the collection/table
- `model` (gardb.Model): Field definitions and validation rules
- `indexes` (gardb.Indexes): Index definitions for searchable fields

Field Types:

```go
schema.String()      // String field
schema.Int()         // Integer field
schema.Float()       // Float field
schema.Bool()        // Boolean field
schema.Time()        // Time/date field
schema.Bytes()       // Binary data
```

Validation Modifiers:

```go
.Required()         // Field cannot be empty
.Default(value)     // Default value if not provided
```

More validation options will be added in future releases.

Example:

```go
type Product struct {
    gardb.GardbBase
    Name        string  `gardb:"name"`
    Description string  `gardb:"description"`
    Price       float64 `gardb:"price"`
    InStock     bool    `gardb:"in_stock"`
}

productSchema, err := gardb.Schema[*Product](ctx, client, "products",
    gardb.Model{
        "name":        schema.String().Required(),
        "description": schema.String(),
        "price":       schema.Float().Required(),
        "in_stock":    schema.Bool().Default(true),
    },
    gardb.Indexes{
        gardb.Index(gardb.Hash("name"), nil),
    },
)
```

### CRUD Operations

#### `Put(ctx, obj *T) error`

Creates a new object or updates an existing one.

Behavior:

- If object.ID is empty → Creates new object (generates ID)
- If object.ID is set → Updates existing object (requires version match for safety)

Example:

```go
// Create
user := &User{
    Email: "bob@example.com",
    Name:  "Bob Jones",
    Age:   35,
}
err := userSchema.Put(ctx, user)
// user.ID now populated: "obj_abc123..."

// Update
user.Age = 36
err = userSchema.Put(ctx, user) // Version checked automatically
```

<hr>

#### `Get(ctx, id string) (T, error)`

Retrieves an object by its ID.

Example:

```go
user, err := userSchema.Get(ctx, "obj_abc123")
if err != nil {
    if errors.Is(err, gardb.ErrNotFound) {
        fmt.Println("User not found")
    }
    return err
}

fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
```

<hr>

#### `Delete(ctx, id string) error`

Soft-deletes an object by its ID (sets its status to "deleted").

Example:

```go
err := userSchema.Delete(ctx, "obj_abc123")
if err != nil {
    return err
}
```

Note: Deleted objects are excluded from queries but can potentially be recovered if needed within 30 days.

<hr>

#### `Scan(ctx, input *ScanInput) (*ScanOutput[T], error)`

Retrieves all objects in a table (use with caution on large datasets).

Example:

```go
var users []*User
err := userSchema.Scan(ctx, &users, &gardb.ScanInput{
    Limit: 100,
})

for _, user := range users {
    fmt.Printf("User: %s\n", user.Name)
}
```

With pagination:

```go
var allUsers []*User
var cursor *string

for {
    out, err := userSchema.Scan(ctx, &gardb.ScanInput{
        Limit:     100,
        Cursor: cursor,
    })
    if err != nil {
        return err
    }

    allUsers = append(allUsers, out.Items...)

    if out.NextCursor == nil {
        break // No more results
    }
    cursor = out.NextCursor
}
```

<hr>

#### Querying

Gardbase supports efficient querying on indexed fields using encrypted search.

##### Simple Query (Hash Index):

```go
// Find users by email
out, err := userSchema.Query(ctx).
    WhereHash("email", gardb.Eq("alice@example.com")).
    Execute()
```

##### Composite Query (Hash + Range Index):

```go
type Book struct {
    gardb.GardbBase
    Author      string    `gardb:"author"`
    PublishedAt time.Time `gardb:"published_at"`
    Title       string    `gardb:"title"`
}

bookSchema, _ := gardb.Schema[*Book](ctx, client, "books",
    gardb.Model{
        "author":       schema.String().Required(),
        "published_at": schema.Time().Required(),
        "title":        schema.String().Required(),
    },
    gardb.Indexes{
        // Composite index: author (hash) + published_at (range)
        gardb.Index(gardb.Hash("author"), gardb.Range("published_at")),
    },
)

// Query books by author published after a certain date
out, err := bookSchema.Query(ctx).
    WhereHash("author", gardb.Eq("Martin Fowler")).
    WhereRange("published_at", gardb.Gt(
        time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
    )).
    Execute()
```

##### Range Conditions

```go
// Exact match
gardb.Eq(value)

// Comparisons
gardb.Lt(value)
gardb.Lte(value)
gardb.Gt(value)
gardb.Gte(value)

// Range
gardb.Between(start, end)
```

##### Query Options

```go
result, err := userSchema.Query(ctx).
    WhereHash("status", gardb.Eq("active")).
    Limit(50).                    // Max results per page
    OrderBy(false).               // false = descending, true = ascending
    StartFrom(cursor).            // Pagination cursor
    Execute()

fmt.Printf("Found %d users\n", len(result.Items))
if result.NextCursor != nil {
    fmt.Println("More results available")
}
```

<hr>

#### `Update(ctx, id string, mutateFn func(dest T) error, opts ...gardb.UpdateOption) (T, error)`

Safely updates an object with automatic retry on version conflicts.

**Why use Update() instead of Get+Put?**

- Automatic retry on conflicts
- Cleaner, more concise code
- Built-in optimistic locking

Example:

```go
user, err := userSchema.Update(ctx, userId, func(u *User) error {
    u.Age = 30
    u.Email = "newemail@example.com"
    return nil
})
if err != nil {
    return err
}
```

With options:

```go
user, err := userSchema.Update(ctx, userId,
    func(u *User) error {
        u.Age = 30
        return nil
    },
    gardb.WithMaxRetries(5),
    gardb.WithRetryDelay(200 * time.Millisecond),
)
```

How it works:

1. Fetches latest version of object
2. Applies your mutation function
3. Attempts to save with version check
4. If version conflict → automatically retries from step 1
5. Returns updated object on success

### Advanced Usage

#### Indexes

Indexes enable efficient querying on specific fields. Without an index, you must use Scan() which reads the entire table.

##### Index Types

1. Hash Index (Equality only)

```go
gardb.Indexes{
    gardb.Index(gardb.Hash("email"), nil),
}

// Can query: email = "alice@example.com"
// Cannot query: email > "a" or email LIKE "%alice%"
```

2. Composite Index (Hash + Range)

```go
gardb.Indexes{
    gardb.Index(
        gardb.Hash("status"),      // Hash key
        gardb.Range("created_at"), // Range key
    ),
}

// Can query:
// - status = "active"
// - status = "active" AND created_at > date
// - status = "active" AND created_at BETWEEN date1 AND date2
```

##### Index Limitations

- Encrypted Search - Indexes use deterministic encryption, which enables exact matches but:
    - No partial matches (LIKE, CONTAINS)
    - No case-insensitive search
    - Limited to indexed fields

- Query Requirements:
    - Hash key must always be specified (exact match)
    - Range key is optional but adds filtering capability

- Storage Cost - Each index creates additional DynamoDB items

#### Composite Indexes

Composite indexes enable queries on multiple fields efficiently.

Example: E-commerce Orders

```go
type Order struct {
    gardb.GardbBase
    UserID      string    `gardb:"user_id"`
    Status      string    `gardb:"status"`
    TotalAmount float64   `gardb:"total_amount"`
    OrderDate   time.Time `gardb:"order_date"`
}

orderSchema, _ := gardb.Schema[*Order](ctx, client, "orders",
    gardb.Model{
        "user_id":      schema.String().Required(),
        "status":       schema.String().Required(),
        "total_amount": schema.Float().Required(),
        "order_date":   schema.Time().Required(),
    },
    gardb.Indexes{
        // Query orders by user + date range
        gardb.Index(
            gardb.Hash("user_id"),
            gardb.Range("order_date"),
        ),
        // Query orders by status + date range
        gardb.Index(
            gardb.Hash("status"),
            gardb.Range("order_date"),
        ),
    },
)

// Query: All orders for a user in the last 30 days
out, err := orderSchema.Query(ctx).
    WhereHash("user_id", gardb.Eq("user_123")).
    WhereRange("order_date", gardb.Gt(
        time.Now().AddDate(0, 0, -30),
    )).
    OrderBy(false). // Newest first
    Execute()

// Query: All pending orders created this month
out, err = orderSchema.Query(ctx).
    WhereHash("status", gardb.Eq("pending")).
    WhereRange("order_date", gardb.Between(
        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
        time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
    )).
    Execute()
```

#### Pagination

All query operations support cursor-based pagination.

```go
var allResults []*User
var cursor *string

for {
    result, err := userSchema.Query(ctx).
        WhereHash("status", gardb.Eq("active")).
        Limit(100).
        StartFrom(cursor).
        Execute()
    if err != nil {
        return err
    }

    allResults = append(allResults, result.Items...)

    if result.NextCursor == nil {
        break // No more pages
    }
    cursor = result.NextCursor
}

fmt.Printf("Total results: %d\n", len(allResults))
```

## Security Model

### Encryption Hierarchy

Gardbase uses a 4-level key hierarchy:

```
Level 1: AWS KMS Key (per environment)
           ↓ wraps
Level 2: Tenant Master Key (per tenant, random 32 bytes)
           ↓ encrypts
Level 3: Data Encryption Keys (DEKs, per object, random 32 bytes)
           ↓ encrypts
Level 4: Your Data (encrypted with DEK using AES-256-GCM)
```

### Properties

- Master Key stored encrypted (KMS-wrapped) on server
- Master Key only decrypted inside AWS Nitro Enclave
- DEKs generated fresh for each object
- All encryption happens client-side or in enclave
- Server never sees plaintext data or unencrypted keys

### Enclave Attestation

Every cryptographic operation goes through a verified AWS Nitro Enclave:

- Client requests enclave session
- Enclave provides cryptographic attestation document
- Client verifies attestation (proves code running in genuine enclave)
- Encrypted channel established
- Enclave performs key operations
- Keys never leave enclave in plaintext

What this means:

- Even Gardbase operators cannot access your keys
- Compromised backend cannot decrypt data
- Hardware-level isolation for sensitive operations

### Index Token Security

Searchable indexes use deterministic encryption:

- Same value always produces same encrypted token
- Enables equality queries on encrypted data
- Index tokens generated in enclave, never exposed
- Cannot reverse token back to original value

**Trade-off**: Deterministic encryption reveals if two records have the same value for an indexed field. Don't index highly sensitive fields if this is a concern.

## Examples

### Example 1: User Management System

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/qodesrl/gardbase-sdk-go/gardb"
    "github.com/qodesrl/gardbase-sdk-go/schema"
)

type User struct {
    gardb.GardbBase
    Email       string    `gardb:"email"`
    Username    string    `gardb:"username"`
    FullName    string    `gardb:"full_name"`
    DateOfBirth time.Time `gardb:"date_of_birth"`
    IsActive    bool      `gardb:"is_active"`
}

func main() {
    ctx := context.Background()

    client, err := gardb.NewClient(&gardb.Config{
        APIEndpoint: "https://api.gardbase.com",
        APIKey:      os.Getenv("GARDBASE_API_KEY"),
        TenantID:    os.Getenv("GARDBASE_TENANT_ID"),
    })
    if err != nil {
        log.Fatal(err)
    }

    userSchema, err := gardb.Schema[*User](ctx, client, "users",
        gardb.Model{
            "email":         schema.String().Required(),
            "username":      schema.String().Required(),
            "full_name":     schema.String().Required(),
            "date_of_birth": schema.Time().Required(),
            "is_active":     schema.Bool().Required(),
        },
        gardb.Indexes{
            gardb.Index(gardb.Hash("email"), nil),
            gardb.Index(gardb.Hash("username"), nil),
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create user
    user := &User{
        Email:       "alice@example.com",
        Username:    "alice123",
        FullName:    "Alice Smith",
        DateOfBirth: time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC),
        IsActive:    true,
    }

    err = userSchema.Put(ctx, user)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created user: %s\n", user.ID)

    // Find by email
    out, err := userSchema.Query(ctx).
        WhereHash("email", gardb.Eq("alice@example.com")).
        Execute()
    if err != nil {
        log.Fatal(err)
    }

    if out.Count > 0 {
        fmt.Printf("Found user: %s (%s)\n", out.Items[0].FullName, out.Items[0].Email)
    }

    // Update user
    updatedUser, err := userSchema.Update(ctx, user.ID, func(u *User) error {
        u.FullName = "Alice Johnson" // Name changed
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Updated name to: %s\n", updatedUser.FullName)
}
```

### Example 2: Blog Post System

```go
type BlogPost struct {
    gardb.GardbBase
    Title       string    `gardb:"title"`
    Content     string    `gardb:"content"`
    AuthorID    string    `gardb:"author_id"`
    Status      string    `gardb:"status"` // "draft", "published"
    PublishedAt time.Time `gardb:"published_at"`
}

postSchema, _ := gardb.Schema[*BlogPost](ctx, client, "posts",
    gardb.Model{
        "title":        schema.String().Required(),
        "content":      schema.String().Required(),
        "author_id":    schema.String().Required(),
        "status":       schema.String().Required(),
        "published_at": schema.Time(),
    },
    gardb.Indexes{
        // Query posts by author
        gardb.Index(gardb.Hash("author_id"), nil),
        // Query published posts by date
        gardb.Index(
            gardb.Hash("status"),
            gardb.Range("published_at"),
        ),
    },
)

// Create draft post
post := &BlogPost{
    Title:    "Getting Started with Gardbase",
    Content:  "Gardbase is a zero-trust encrypted database...",
    AuthorID: authorID,
    Status:   "draft",
}
postSchema.Put(ctx, post)

// Publish post
postSchema.Update(ctx, post.ID, func(p *BlogPost) error {
    p.Status = "published"
    p.PublishedAt = time.Now()
    return nil
})

// Get recent published posts
recentPosts, _ := postSchema.Query(ctx).
    WhereHash("status", gardb.Eq("published")).
    WhereRange("published_at", gardb.Gt(
        time.Now().AddDate(0, 0, -7), // Last 7 days
    )).
    OrderBy(false). // Newest first
    Limit(10).
    Execute()

for _, post := range recentPosts {
    fmt.Printf("📝 %s (by %s)\n", post.Title, post.AuthorID)
}
```

### Example 3: E-commerce Orders

```go
type Order struct {
    gardb.GardbBase
    UserID      string    `gardb:"user_id"`
    Status      string    `gardb:"status"` // "pending", "paid", "shipped", "delivered"
    TotalAmount float64   `gardb:"total_amount"`
    OrderDate   time.Time `gardb:"order_date"`
    ShippedDate time.Time `gardb:"shipped_date"`
}

orderSchema, _ := gardb.Schema[*Order](ctx, client, "orders",
    gardb.Model{
        "user_id":      schema.String().Required(),
        "status":       schema.String().Required(),
        "total_amount": schema.Float().Required().Min(0),
        "order_date":   schema.Time().Required(),
        "shipped_date": schema.Time(),
    },
    gardb.Indexes{
        gardb.Index(
            gardb.Hash("user_id"),
            gardb.Range("order_date"),
        ),
        gardb.Index(
            gardb.Hash("status"),
            gardb.Range("order_date"),
        ),
    },
)

// Create order
order := &Order{
    UserID:      "user_123",
    Status:      "pending",
    TotalAmount: 99.99,
    OrderDate: time.Now(),
}
orderSchema.Put(ctx, order)

// Ship order
orderSchema.Update(ctx, order.ID, func(o *Order) error {
    o.Status = "shipped"
    o.ShippedDate = time.Now()
    return nil
})

// Get user's order history
orders, _ := orderSchema.Query(ctx).
    WhereHash("user_id", gardb.Eq("user_123")).
    WhereRange("order_date", gardb.Gt(
        time.Now().AddDate(0, -3, 0), // Last 3 months
    )).
    OrderBy(false). // Newest first
    Execute()
```

## Contributing

We welcome contributions! Please fork the repository and submit a pull request with your changes. For major changes, please open an issue first to discuss what you would like to change.

## License

The project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
