package gardb

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/internal"
)

type Client struct {
	apiClient     *internal.APIClient
	enclaveClient *internal.EnclaveClient
	config        *Config
}

type Config struct {
	// Required
	APIEndpoint string
	KMSKeyID    string

	// Attestation verification
	ExpectedPCRs      map[uint]string
	VerifyAttestation bool // default: true
	RootCA            *x509.Certificate
	VerifyPCRs        bool // UNSAFE: only for local dev

	// Optional
	HTTPTimeout       time.Duration // default: 30 s
	MaxAttestationAge time.Duration // default: 5 min

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

func NewClient(config *Config) (*Client, *Error) {
	if config == nil {
		return nil, &Error{Op: "NewClient", Err: ErrInvalidConfig}
	}
	if config.APIEndpoint == "" {
		return nil, &Error{Op: "NewClient", Err: fmt.Errorf("APIEndpoint is required")}
	}
	if config.KMSKeyID == "" {
		return nil, &Error{Op: "NewClient", Err: fmt.Errorf("KMSKeyID is required")}
	}

	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 30 * time.Second
	}
	if config.MaxAttestationAge == 0 {
		config.MaxAttestationAge = 5 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 2.0
	}
	if config.SessionRenewalThreshold == 0 {
		config.SessionRenewalThreshold = 5 * time.Minute
	}
	if config.Logger == nil {
		config.Logger = &defaultLogger{}
	}

	if config.VerifyAttestation {
		if len(config.ExpectedPCRs) == 0 {
			return nil, &Error{Op: "NewClient", Err: fmt.Errorf("ExpectedPCRs must be set when VerifyAttestation is true")}
		}
	}

	enclaveClient := &internal.EnclaveClient{
		APIEndpoint:             config.APIEndpoint,
		KMSKeyID:                config.KMSKeyID,
		ExpectedPCRs:            config.ExpectedPCRs,
		VerifyAttestation:       config.VerifyAttestation,
		RootCA:                  config.RootCA,
		VerifyPCRs:              config.VerifyPCRs,
		MaxAttestationAge:       config.MaxAttestationAge,
		HTTPTimeout:             config.HTTPTimeout,
		SessionRenewalThreshold: config.SessionRenewalThreshold,
	}

	apiClient := internal.NewAPIClient(config.APIEndpoint, config.HTTPTimeout)

	client := &Client{
		config:        config,
		enclaveClient: enclaveClient,
		apiClient:     apiClient,
	}

	return client, nil
}

func (c *Client) Close() error {
	return nil
}
