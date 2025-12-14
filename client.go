package gardbase

import (
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/internal"
)

type Client struct {
	apiClient     *internal.APIClient
	enclaveClient *internal.EnclaveClient
	config        *Config
}

type Config struct {
	APIEndpoint string
	KMSKeyID    string

	ExpectedPCRs      map[uint]string
	VerifyAttestation bool // default: true

	// Optional
	HTTPTimeout time.Duration
	MaxRetries  int
	SessionTTL  time.Duration
}

func NewClient(config *Config) (*Client, error) {

}

func (c *Client) Close() error {

}
