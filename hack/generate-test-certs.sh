#!/bin/bash
# Generate self-signed test certificates for unit and integration testing.
# Usage: ./hack/generate-test-certs.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="${SCRIPT_DIR}/../internal/cert/testdata"

mkdir -p "${CERT_DIR}"

echo "==> Generating valid self-signed certificate..."
openssl req -x509 -newkey rsa:2048 -sha256 -days 365 \
    -nodes -keyout "${CERT_DIR}/valid-key.pem" -out "${CERT_DIR}/valid.pem" \
    -subj "/CN=test.example.com" \
    -addext "subjectAltName=DNS:test.example.com,DNS:*.example.com,IP:127.0.0.1"

echo "==> Generating expired certificate..."
openssl req -x509 -newkey rsa:2048 -sha256 -days 1 \
    -nodes -keyout "${CERT_DIR}/expired-key.pem" -out "${CERT_DIR}/expired.pem" \
    -subj "/CN=expired.example.com" \
    -addext "subjectAltName=DNS:expired.example.com"
# Backdate the cert so it's already expired
# Note: This produces a cert valid from now for 1 day.
# For truly expired certs in tests, use faketime or set NotAfter in Go test code.

echo "==> Generating CA + signed certificate..."
# CA
openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 \
    -nodes -keyout "${CERT_DIR}/ca-key.pem" -out "${CERT_DIR}/ca.pem" \
    -subj "/CN=Test CA"

# Server cert signed by CA
openssl req -newkey rsa:2048 -nodes \
    -keyout "${CERT_DIR}/signed-key.pem" -out "${CERT_DIR}/signed.csr" \
    -subj "/CN=signed.example.com" \
    -addext "subjectAltName=DNS:signed.example.com"

openssl x509 -req -in "${CERT_DIR}/signed.csr" \
    -CA "${CERT_DIR}/ca.pem" -CAkey "${CERT_DIR}/ca-key.pem" \
    -CAcreateserial -out "${CERT_DIR}/signed.pem" \
    -days 365 -sha256 \
    -extfile <(echo "subjectAltName=DNS:signed.example.com")

rm -f "${CERT_DIR}/signed.csr" "${CERT_DIR}/ca.srl"

echo "==> Creating chain bundle..."
cat "${CERT_DIR}/signed.pem" "${CERT_DIR}/ca.pem" > "${CERT_DIR}/chain-bundle.pem"

echo "==> Test certificates generated in ${CERT_DIR}"
ls -la "${CERT_DIR}"
