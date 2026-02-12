package gardb

import (
	"context"
	"testing"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/schema"
)

type testLogger struct{}

func (l *testLogger) Debug(msg string, args ...any) {}
func (l *testLogger) Info(msg string, args ...any)  {}
func (l *testLogger) Warn(msg string, args ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {}

func TestNewClient_NilConfig(t *testing.T) {
	cli, err := NewClient(nil)
	if err == nil {
		t.Fatalf("expected error for nil config, got nil")
	}
	if cli != nil {
		t.Fatalf("expected nil client on error, got %v", cli)
	}
}

func TestNewClient_MissingRequiredFields(t *testing.T) {
	// Missing APIEndpoint
	_, err := NewClient(&Config{APIKey: "key", TenantID: "tenant"})
	if err == nil {
		t.Fatalf("expected error when APIEndpoint is missing")
	}
	// Missing APIKey
	_, err = NewClient(&Config{APIEndpoint: "https://api", TenantID: "tenant"})
	if err == nil {
		t.Fatalf("expected error when APIKey is missing")
	}
	// Missing TenantID
	_, err = NewClient(&Config{APIEndpoint: "https://api", APIKey: "key"})
	if err == nil {
		t.Fatalf("expected error when TenantID is missing")
	}
}

func TestNewClient_DefaultsApplied(t *testing.T) {
	cfg := &Config{
		APIEndpoint: "https://api.example",
		APIKey:      "api-key",
		TenantID:    "tenant-123",
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatalf("expected non-nil client")
	}
	if c.config.HTTPTimeout != 30*time.Second {
		t.Fatalf("HTTPTimeout default mismatch: got %v", c.config.HTTPTimeout)
	}
	if c.config.MaxAttestationAge != 5*time.Minute {
		t.Fatalf("MaxAttestationAge default mismatch: got %v", c.config.MaxAttestationAge)
	}
	if c.config.MaxRetries != 3 {
		t.Fatalf("MaxRetries default mismatch: got %d", c.config.MaxRetries)
	}
	if c.config.RetryDelay != 1*time.Second {
		t.Fatalf("RetryDelay default mismatch: got %v", c.config.RetryDelay)
	}
	if c.config.RetryBackoff != 2.0 {
		t.Fatalf("RetryBackoff default mismatch: got %v", c.config.RetryBackoff)
	}
	if c.config.SessionRenewalThreshold != 5*time.Minute {
		t.Fatalf("SessionRenewalThreshold default mismatch: got %v", c.config.SessionRenewalThreshold)
	}
	if c.config.Logger == nil {
		t.Fatalf("expected default logger to be set")
	}
	if _, ok := c.config.Logger.(*defaultLogger); !ok {
		t.Fatalf("expected default logger type, got %T", c.config.Logger)
	}
	if c.enclaveClient == nil {
		t.Fatalf("expected enclaveClient to be initialized")
	}
	if c.apiClient == nil {
		t.Fatalf("expected apiClient to be initialized")
	}
}

func TestNewClient_CustomConfigPreservedAndCopied(t *testing.T) {
	logger := &testLogger{}
	cfg := &Config{
		APIEndpoint:             "https://api.custom",
		APIKey:                  "custom-key",
		TenantID:                "tenant-x",
		HTTPTimeout:             10 * time.Second,
		MaxAttestationAge:       2 * time.Minute,
		MaxRetries:              5,
		RetryDelay:              500 * time.Millisecond,
		RetryBackoff:            1.5,
		SessionRenewalThreshold: 1 * time.Minute,
		Logger:                  logger,
		VerifyAttestation:       false,
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.config.HTTPTimeout != 10*time.Second {
		t.Fatalf("HTTPTimeout mismatch: got %v", c.config.HTTPTimeout)
	}
	if c.config.MaxRetries != 5 {
		t.Fatalf("MaxRetries mismatch: got %d", c.config.MaxRetries)
	}
	if c.config.RetryBackoff != 1.5 {
		t.Fatalf("RetryBackoff mismatch: got %v", c.config.RetryBackoff)
	}
	if c.config.Logger != logger {
		t.Fatalf("Logger not preserved")
	}
	// Ensure original config modifications do not affect client (cfg was copied)
	cfg.HTTPTimeout = 1 * time.Nanosecond
	if c.config.HTTPTimeout == cfg.HTTPTimeout {
		t.Fatalf("client config should not reflect changes to original config after NewClient")
	}
}

func TestSchema_ValidationErrors(t *testing.T) {
	cfg := &Config{
		APIEndpoint: "https://api.example",
		APIKey:      "api-key",
		TenantID:    "tenant-123",
	}
	c, err := NewClient(cfg)
	ctx := context.Background()
	type Obj struct {
		GardbBase
		F string `gardb:"f"`
	}
	// Empty name
	_, err = Schema[*Obj](ctx, c, "", Model{"f": schema.String()})
	if err == nil {
		t.Fatalf("expected error for empty schema name")
	}
	// Empty model
	_, err = Schema[*Obj](ctx, c, "users", Model{})
	if err == nil {
		t.Fatalf("expected error for empty schema model")
	}
}
