package gardb

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase-sdk-go/internal"
	"github.com/QodeSrl/gardbase-sdk-go/schema"
	"github.com/QodeSrl/gardbase/pkg/api/objects"
)

type Client struct {
	mu            sync.RWMutex
	apiClient     *internal.APIClient
	enclaveClient *internal.EnclaveClient
	cache         *internal.Cache
	config        *Config
}

type Config struct {
	// Required
	APIEndpoint string
	APIKey      string
	TenantID    string

	// Attestation verification
	ExpectedPCRs        map[uint]string
	VerifyAttestation   bool // default: true
	RootCA              *x509.Certificate
	SkipPCRVerification bool // UNSAFE: only for local dev

	// Optional
	HTTPTimeout       time.Duration // default: 30 s
	MaxAttestationAge time.Duration // default: 5 min
	CacheDir          string        // path to cache directory, default: OS temp dir

	// Retry settings
	MaxRetries   int           // default: 3
	RetryDelay   time.Duration // default: 1 s
	RetryBackoff float64       // default: 2.0

	// Logging
	Logger Logger // optional logger interface

	// Session renewal
	SessionRenewalThreshold time.Duration // renew sessions when this close to expiry - default: 5 min
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...any) {}
func (l *defaultLogger) Info(msg string, args ...any)  {}
func (l *defaultLogger) Warn(msg string, args ...any)  {}
func (l *defaultLogger) Error(msg string, args ...any) {}

func NewClient(config *Config) (*Client, error) {
	const op = "NewClient"

	if config == nil {
		return nil, errors.ConfigError(op, "config is required")
	}
	if config.APIEndpoint == "" {
		return nil, errors.ConfigError(op, "APIEndpoint is required")
	}
	if config.APIKey == "" {
		return nil, errors.ConfigError(op, "APIKey is required")
	}
	if config.TenantID == "" {
		return nil, errors.ConfigError(op, "TenantID is required")
	}

	cfgCpy := *config

	if cfgCpy.HTTPTimeout == 0 {
		cfgCpy.HTTPTimeout = 30 * time.Second
	}
	if cfgCpy.MaxAttestationAge == 0 {
		cfgCpy.MaxAttestationAge = 5 * time.Minute
	}
	if cfgCpy.CacheDir == "" {
		cfgCpy.CacheDir = os.TempDir()
	}
	if cfgCpy.MaxRetries == 0 {
		cfgCpy.MaxRetries = 3
	}
	if cfgCpy.RetryDelay == 0 {
		cfgCpy.RetryDelay = 1 * time.Second
	}
	if cfgCpy.RetryBackoff == 0 {
		cfgCpy.RetryBackoff = 2.0
	}
	if cfgCpy.SessionRenewalThreshold == 0 {
		cfgCpy.SessionRenewalThreshold = 5 * time.Minute
	}
	if cfgCpy.Logger == nil {
		cfgCpy.Logger = &defaultLogger{}
	}

	cache, err := internal.NewCache(cfgCpy.CacheDir)
	if err != nil {
		return nil, err
	}

	httpClient := internal.NewHttpClient(cfgCpy.TenantID, cfgCpy.APIKey, cfgCpy.HTTPTimeout)

	enclaveClient := &internal.EnclaveClient{
		APIEndpoint:             cfgCpy.APIEndpoint + "/api",
		TenantID:                cfgCpy.TenantID,
		APIKey:                  cfgCpy.APIKey,
		HttpClient:              httpClient,
		ExpectedPCRs:            cfgCpy.ExpectedPCRs,
		VerifyAttestation:       cfgCpy.VerifyAttestation,
		RootCA:                  cfgCpy.RootCA,
		SkipPCRVerification:     cfgCpy.SkipPCRVerification,
		MaxAttestationAge:       cfgCpy.MaxAttestationAge,
		HTTPTimeout:             cfgCpy.HTTPTimeout,
		SessionRenewalThreshold: cfgCpy.SessionRenewalThreshold,
	}

	apiClient := internal.NewAPIClient(cfgCpy.APIEndpoint, cfgCpy.TenantID, cfgCpy.APIKey, httpClient)

	client := &Client{
		mu:            sync.RWMutex{},
		config:        &cfgCpy,
		enclaveClient: enclaveClient,
		apiClient:     apiClient,
		cache:         cache,
	}

	return client, nil
}

func (c *Client) Close() error {
	return c.enclaveClient.Close()
}

func Schema[T GardbObject](ctx context.Context, client *Client, name string, model Model, indexes Indexes) (*GardbSchema[T], error) {
	const op = "Schema"

	if name == "" {
		return nil, &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: schema name cannot be empty", errors.ErrInvalidSchema),
		}
	}

	if len(model) == 0 {
		return nil, &errors.Error{
			Op:  op,
			Err: fmt.Errorf("%w: schema model cannot be empty", errors.ErrInvalidSchema),
		}
	}

	fields := make(map[string]*schema.Field, len(model))
	timeFields := make([]string, 0)

	for fieldName, field := range model {
		if fieldName == "" {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: field name cannot be empty", errors.ErrInvalidSchema),
			}
		}
		field.Name = fieldName
		fields[fieldName] = field

		if field.Typ == schema.TimeType {
			timeFields = append(timeFields, fieldName)
		}
	}

	for _, idx := range indexes {
		if idx.HashField == "" {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: index hash key cannot be empty", errors.ErrInvalidSchema),
			}
		}
		if _, ok := fields[idx.HashField]; !ok {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: index hash key '%s' not found in model fields", errors.ErrInvalidSchema, idx.HashField),
			}
		}
		if !slices.Contains([]schema.FieldType{schema.StringType, schema.IntegerType, schema.BooleanType, schema.TimeType}, fields[idx.HashField].Typ) {
			return nil, &errors.Error{
				Op:  op,
				Err: fmt.Errorf("%w: index hash key '%s' must be of type string, int, bool, or time", errors.ErrInvalidSchema, idx.HashField),
			}
		}
		if idx.RangeField != nil {
			if _, ok := fields[*idx.RangeField]; !ok {
				return nil, &errors.Error{
					Op:  op,
					Err: fmt.Errorf("%w: index range key '%s' not found in model fields", errors.ErrInvalidSchema, *idx.RangeField),
				}
			}
			if !slices.Contains([]schema.FieldType{schema.StringType, schema.IntegerType, schema.BooleanType, schema.FloatType, schema.TimeType}, fields[*idx.RangeField].Typ) {
				return nil, &errors.Error{
					Op:  op,
					Err: fmt.Errorf("%w: index range key '%s' must be of type string, int, bool, float, or time", errors.ErrInvalidSchema, *idx.RangeField),
				}
			}
		}
	}

	tableHash, ok := client.cache.Get("tablehash__" + name)
	if !ok || tableHash == "" || tableHash == nil {
		hash, err := client.enclaveClient.GetTableHash(ctx, name)
		if err != nil {
			return nil, err
		}
		client.cache.Set("tablehash__"+name, hash)
		tableHash = hash
	}

	s := &GardbSchema[T]{
		name:       name,
		tableHash:  tableHash.(string),
		fields:     fields,
		indexes:    indexes,
		timeFields: timeFields,
		client:     client,
		typ:        reflect.TypeOf((*T)(nil)).Elem().Elem(),
	}

	return s, nil
}

func Hash(hashKey string) *objects.IndexName {
	return &objects.IndexName{HashField: hashKey}
}

func Range(rangeKey string) *objects.IndexName {
	return &objects.IndexName{RangeField: &rangeKey}
}

func Index(hashKeyIndex, rangeKeyIndex *objects.IndexName) *objects.IndexName {
	idx := &objects.IndexName{HashField: hashKeyIndex.HashField}
	if rangeKeyIndex != nil {
		idx.RangeField = rangeKeyIndex.RangeField
	}
	return idx
}
