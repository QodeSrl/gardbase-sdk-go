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

func (ec *EnclaveClient) shouldRenewSession(expiresAt time.Time) bool {
	renewalTime := time.Now().Add(ec.SessionRenewalThreshold)
	return expiresAt.Before(renewalTime)
}

func (ec *EnclaveClient) ensureSession(ctx context.Context) error {
	ec.essMu.Lock()
	defer ec.essMu.Unlock()

	if ec.ess != nil {
		if !ec.shouldRenewSession(ec.ess.ExpiresAt) {
			return nil
		}
		ec.ess.Close()
		ec.ess = nil
	}

	err := ec.InitEnclaveSecureSession(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (ec *EnclaveClient) InitEnclaveSecureSession(ctx context.Context) error {
	ec.essMu.Lock()
	defer ec.essMu.Unlock()

	config := crypto.SessionConfig{
		Endpoint:          ec.APIEndpoint + "/enclave",
		ExpectedPCRs:      ec.ExpectedPCRs,
		RootCA:            ec.RootCA,
		MaxAttestationAge: ec.MaxAttestationAge,
		VerifyPCRs:        ec.VerifyPCRs,
		HTTPTimeout:       ec.HTTPTimeout,
	}
	ess, err := crypto.InitEnclaveSecureSession(ctx, config)
	if err != nil {
		return err
	}
	ec.ess = ess
	return nil
}

type SessionInfo struct {
	Active              bool
	SessionID           string
	ExpiresAt           time.Time
	AttestationVerified bool
	TimeToExpiry        time.Duration
}

func (ec *EnclaveClient) GetSessionInfo(ctx context.Context) *SessionInfo {
	ec.essMu.RLock()
	defer ec.essMu.RUnlock()

	if ec.ess == nil {
		return &SessionInfo{
			Active: false,
		}
	}

	return &SessionInfo{
		Active:              true,
		SessionID:           ec.ess.SessionId,
		ExpiresAt:           ec.ess.ExpiresAt,
		AttestationVerified: ec.ess.AttestationVerified,
		TimeToExpiry:        time.Until(ec.ess.ExpiresAt),
	}
}

func (ec *EnclaveClient) GenerateDEK(ctx context.Context, count int) ([]crypto.GeneratedDEK, error) {
	if err := ec.ensureSession(ctx); err != nil {
		return nil, err
	}

	ec.essMu.RLock()
	defer ec.essMu.RUnlock()

	return ec.ess.GenerateDEK(ctx, ec.KMSKeyID, count)
}
