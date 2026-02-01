package internal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase/pkg/api/objects"
	"github.com/QodeSrl/gardbase/pkg/crypto"
	"github.com/QodeSrl/gardbase/pkg/enclaveproto"
	"golang.org/x/crypto/chacha20poly1305"
)

type EnclaveClient struct {
	ess   *crypto.EnclaveSecureSession
	essMu sync.RWMutex

	APIEndpoint string
	TenantID    string
	APIKey      string

	HttpClient *http.Client

	// Attestation Verification
	ExpectedPCRs        map[uint]string
	VerifyAttestation   bool
	RootCA              *x509.Certificate
	SkipPCRVerification bool
	MaxAttestationAge   time.Duration

	HTTPTimeout             time.Duration
	SessionRenewalThreshold time.Duration
}

func (ec *EnclaveClient) Close() error {
	ec.essMu.Lock()
	defer ec.essMu.Unlock()

	if ec.ess != nil {
		ec.ess.Close()
	}

	return nil
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

	err := ec.initEnclaveSecureSessionLocked(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (ec *EnclaveClient) initEnclaveSecureSessionLocked(ctx context.Context) error {
	config := crypto.SessionConfig{
		Endpoint:          ec.APIEndpoint + "/encryption",
		TenantID:          ec.TenantID,
		APIKey:            ec.APIKey,
		ExpectedPCRs:      ec.ExpectedPCRs,
		RootCA:            ec.RootCA,
		MaxAttestationAge: ec.MaxAttestationAge,
		VerifyPCRs:        !ec.SkipPCRVerification,
		HTTPTimeout:       ec.HTTPTimeout,
	}
	ess, err := crypto.InitEnclaveSecureSession(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to initialize enclave secure session: %w", err)
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

	return ec.ess.GenerateDEK(ctx, count)
}

func (ec *EnclaveClient) DecryptDEK(ctx context.Context, objectID string, encryptedDEKB64 string) ([]byte, error) {
	if err := ec.ensureSession(ctx); err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrSession, err)
	}
	ec.essMu.RLock()
	defer ec.essMu.RUnlock()

	item := enclaveproto.SessionUnwrapItem{
		ObjectId:   objectID,
		Ciphertext: encryptedDEKB64,
	}
	items := []enclaveproto.SessionUnwrapItem{item}

	res, err := ec.ess.SessionUnwrap(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrEncryption, err)
	}
	if res[0].Error != "" {
		return nil, fmt.Errorf("%w: %s", errors.ErrEncryption, res[0].Error)
	}

	dek, err := ec.ess.UnsealDEK(ctx, res[0].SealedDEK, res[0].Nonce, objectID)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to unseal DEK: %v", errors.ErrEncryption, err)
	}
	return dek, nil
}

func (ec *EnclaveClient) GetTableHash(ctx context.Context, tableName string) (string, error) {
	if err := ec.ensureSession(ctx); err != nil {
		return "", fmt.Errorf("%w: %w", errors.ErrSession, err)
	}
	ec.essMu.RLock()
	defer ec.essMu.RUnlock()

	// Encrypt table name with shared session key
	aead, err := chacha20poly1305.NewX(ec.ess.SessionKey)
	if err != nil {
		return "", fmt.Errorf("%w: failed to create AEAD cipher: %v", errors.ErrEncryption, err)
	}

	nonce := make([]byte, chacha20poly1305.NonceSizeX) // 24 bytes
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("%w: failed to generate nonce: %v", errors.ErrEncryption, err)
	}
	encryptedTableName := aead.Seal(nil, nonce, []byte(tableName), nil)

	reqBody, err := json.Marshal(objects.GetTableHashRequest{
		SessionID:                 ec.ess.SessionId,
		SessionEncryptedTableName: base64.StdEncoding.EncodeToString(encryptedTableName),
		SessionTableNameNonce:     base64.StdEncoding.EncodeToString(nonce),
	})
	if err != nil {
		return "", fmt.Errorf("%w: failed to marshal request body: %v", errors.ErrValidation, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ec.APIEndpoint+"/objects/table-hash", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("%w: failed to create request: %v", errors.ErrValidation, err)
	}
	resp, err := ec.HttpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: failed to perform request: %v", errors.ErrNetwork, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: failed to get table hash, status code: %d", errors.ErrNetwork, resp.StatusCode)
	}
	var getTableHashResp objects.GetTableHashResponse
	if err := json.NewDecoder(resp.Body).Decode(&getTableHashResp); err != nil {
		return "", fmt.Errorf("%w: failed to decode response body: %v", errors.ErrNetwork, err)
	}
	return getTableHashResp.TableHash, nil
}
