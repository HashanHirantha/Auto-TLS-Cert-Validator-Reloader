// Package cert provides X.509 certificate parsing, validation, and fingerprinting.
package cert

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"
)

// ValidationResult captures the outcome of a certificate validation.
type ValidationResult struct {
	// Valid is true when all requested checks pass.
	Valid bool

	// Certificate is the parsed X.509 certificate (nil on parse failure).
	Certificate *x509.Certificate

	// Fingerprint is the SHA-256 hex digest of the certificate DER bytes.
	Fingerprint string

	// ExpiresAt is the certificate's NotAfter time.
	ExpiresAt time.Time

	// DaysUntilExpiry is the number of days until the certificate expires.
	DaysUntilExpiry int

	// Errors collects all validation errors encountered.
	Errors []string
}

// ValidateOptions configures what checks to perform.
type ValidateOptions struct {
	CheckExpiry         bool
	ExpiryThresholdDays int
	CheckChain          bool
	CheckHostnames      []string
	CACertPEM           []byte // optional CA bundle for chain verification
}

// Validate parses and validates a PEM-encoded X.509 certificate.
func Validate(certPEM []byte, opts ValidateOptions) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Decode PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "failed to decode PEM block")
		return result
	}

	// Parse X.509
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to parse certificate: %v", err))
		return result
	}

	result.Certificate = cert
	result.ExpiresAt = cert.NotAfter
	result.DaysUntilExpiry = int(time.Until(cert.NotAfter).Hours() / 24)

	// Compute fingerprint
	hash := sha256.Sum256(cert.Raw)
	result.Fingerprint = hex.EncodeToString(hash[:])

	// Check expiry
	if opts.CheckExpiry {
		if time.Now().After(cert.NotAfter) {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("certificate expired at %s", cert.NotAfter.Format(time.RFC3339)))
		} else if opts.ExpiryThresholdDays > 0 {
			threshold := time.Duration(opts.ExpiryThresholdDays) * 24 * time.Hour
			if time.Until(cert.NotAfter) < threshold {
				// Not invalid, but add a warning
				result.Errors = append(result.Errors,
					fmt.Sprintf("certificate expires in %d days (threshold: %d)",
						result.DaysUntilExpiry, opts.ExpiryThresholdDays))
			}
		}
	}

	// Check hostnames
	for _, hostname := range opts.CheckHostnames {
		if err := cert.VerifyHostname(hostname); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("hostname verification failed for %q: %v", hostname, err))
		}
	}

	// Check chain
	if opts.CheckChain {
		roots := x509.NewCertPool()
		if len(opts.CACertPEM) > 0 {
			roots.AppendCertsFromPEM(opts.CACertPEM)
		} else {
			// For self-signed certs, use the cert itself as a root
			roots.AddCert(cert)
		}
		verifyOpts := x509.VerifyOptions{
			Roots: roots,
		}
		if _, err := cert.Verify(verifyOpts); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("chain verification failed: %v", err))
		}
	}

	return result
}

// Fingerprint computes the SHA-256 fingerprint of a PEM-encoded certificate.
func Fingerprint(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:]), nil
}
