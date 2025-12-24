package gardb

import "fmt"

type Error struct {
	Op  string // operation
	Err error  // underlying error
}

func (e *Error) Error() string {
	if e.Op == "" {
		return fmt.Sprintf("gardbase: %v", e.Err)
	}
	return fmt.Sprintf("gardbase %s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Common errors
var (
	// Client errors
	ErrInvalidConfig   = fmt.Errorf("invalid configuration")
	ErrClientClosed    = fmt.Errorf("client is closed")
	ErrInvalidObjectID = fmt.Errorf("invalid object ID")
	ErrInvalidMetadata = fmt.Errorf("invalid metadata")

	// Network errors
	ErrNetworkTimeout     = fmt.Errorf("network timeout")
	ErrServiceUnavailable = fmt.Errorf("service unavailable")
	ErrRequestFailed      = fmt.Errorf("request failed")

	// Encryption errors
	ErrEncryptionFailed = fmt.Errorf("encryption failed")
	ErrDecryptionFailed = fmt.Errorf("decryption failed")
	ErrInvalidDEK       = fmt.Errorf("invalid DEK")

	// Attestation errors
	ErrAttestationFailed  = fmt.Errorf("attestation verification failed")
	ErrPCRMismatch        = fmt.Errorf("PCR values do not match")
	ErrInvalidAttestation = fmt.Errorf("invalid attestation document")

	// Data errors
	ErrObjectNotFound = fmt.Errorf("object not found")
	ErrObjectTooLarge = fmt.Errorf("object exceeds size limit")
	ErrQueryFailed    = fmt.Errorf("query failed")

	// Session errors
	ErrSessionExpired = fmt.Errorf("session expired")
	ErrSessionInvalid = fmt.Errorf("session invalid")

	// Object errors
	ErrInvalidObjectType = fmt.Errorf("invalid object type; must be pointer to struct with GardbMeta field")
)
