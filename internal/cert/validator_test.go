package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generateTestCert creates a self-signed PEM certificate for testing.
func generateTestCert(t *testing.T, cn string, notBefore, notAfter time.Time, sans []string) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     sans,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func TestValidate_ValidCert(t *testing.T) {
	certPEM := generateTestCert(t,
		"test.example.com",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(365*24*time.Hour),
		[]string{"test.example.com", "*.example.com"},
	)

	result := Validate(certPEM, ValidateOptions{
		CheckExpiry:         true,
		ExpiryThresholdDays: 30,
		CheckChain:          true,
		CheckHostnames:      []string{"test.example.com"},
	})

	if !result.Valid {
		t.Errorf("expected valid cert, got errors: %v", result.Errors)
	}
	if result.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}
	if result.DaysUntilExpiry < 364 {
		t.Errorf("expected ~365 days until expiry, got %d", result.DaysUntilExpiry)
	}
}

func TestValidate_ExpiredCert(t *testing.T) {
	certPEM := generateTestCert(t,
		"expired.example.com",
		time.Now().Add(-48*time.Hour),
		time.Now().Add(-1*time.Hour),
		nil,
	)

	result := Validate(certPEM, ValidateOptions{
		CheckExpiry: true,
	})

	if result.Valid {
		t.Error("expected invalid cert for expired certificate")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error for expired cert")
	}
}

func TestValidate_ExpiryThreshold(t *testing.T) {
	certPEM := generateTestCert(t,
		"expiring.example.com",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(15*24*time.Hour), // Expires in 15 days
		nil,
	)

	result := Validate(certPEM, ValidateOptions{
		CheckExpiry:         true,
		ExpiryThresholdDays: 30, // Threshold is 30 days
	})

	// Cert is still valid but within threshold — should produce a warning
	if !result.Valid {
		t.Error("cert is still valid, should not be marked invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected a warning about approaching expiry")
	}
}

func TestValidate_HostnameMismatch(t *testing.T) {
	certPEM := generateTestCert(t,
		"test.example.com",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(365*24*time.Hour),
		[]string{"test.example.com"},
	)

	result := Validate(certPEM, ValidateOptions{
		CheckHostnames: []string{"wrong.example.com"},
	})

	if result.Valid {
		t.Error("expected invalid cert for hostname mismatch")
	}
}

func TestValidate_InvalidPEM(t *testing.T) {
	result := Validate([]byte("not a valid PEM"), ValidateOptions{})

	if result.Valid {
		t.Error("expected invalid result for garbage PEM data")
	}
	if result.Certificate != nil {
		t.Error("expected nil certificate for invalid PEM")
	}
}

func TestFingerprint(t *testing.T) {
	certPEM := generateTestCert(t,
		"fingerprint.example.com",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(365*24*time.Hour),
		nil,
	)

	fp1, err := Fingerprint(certPEM)
	if err != nil {
		t.Fatalf("fingerprint failed: %v", err)
	}
	if len(fp1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64-char hex fingerprint, got %d chars", len(fp1))
	}

	// Same cert should produce the same fingerprint
	fp2, err := Fingerprint(certPEM)
	if err != nil {
		t.Fatalf("fingerprint failed: %v", err)
	}
	if fp1 != fp2 {
		t.Error("expected same fingerprint for same cert")
	}
}
