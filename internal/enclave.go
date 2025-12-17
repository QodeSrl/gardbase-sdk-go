package internal

import (
	"context"
	"crypto/x509"
	"sync"
	"time"

	"github.com/QodeSrl/gardbase/pkg/crypto"
)

type EnclaveClient struct {
	ess   *crypto.EnclaveSecureSession
	essMu sync.RWMutex

	APIEndpoint string
	KMSKeyID    string

	// Attestation Verification
	ExpectedPCRs      map[uint]string
	VerifyAttestation bool
	RootCA            *x509.Certificate
	VerifyPCRs        bool
	MaxAttestationAge time.Duration

	HTTPTimeout             time.Duration
	SessionRenewalThreshold time.Duration
}
