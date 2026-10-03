# Auto-TLS Cert Validator & Reloader

A Kubernetes Operator written in Go that automatically validates TLS certificates and triggers rolling restarts of workloads when certificates change or become invalid.

## Features

- 🔍 **Certificate Validation** — Parses X.509 certs, checks expiry, verifies chain and SANs
- 🔄 **Automatic Reload** — Rolling restart of Deployments, StatefulSets, and DaemonSets on cert change
- 📊 **Prometheus Metrics** — Expiry countdown, validity status, reload counts
- 🔐 **RBAC Hardened** — Least-privilege ClusterRole with no write access to Secrets
- ⚡ **Level-Triggered** — Idempotent reconciliation loop, resilient to restarts

## Quick Start

```bash
# 1. Create a kind cluster
make kind-cluster

# 2. Install CRDs
make install

# 3. Run the operator locally
make run

# 4. Apply a sample TLSCertWatch resource
kubectl apply -f config/samples/tls_v1alpha1_tlscertwatch.yaml
```

## Architecture

```
TLSCertWatch CR → Controller → Validate Cert → Patch Target → Rolling Restart
                       ↓
                  Status Update + Events + Metrics
```

## Project Structure

```
api/v1alpha1/          CRD type definitions
cmd/                   Operator entrypoint
internal/controller/   Reconciliation loop
internal/cert/         X.509 validation & fingerprinting
internal/reloader/     Workload restart strategies
internal/metrics/      Prometheus metric definitions
config/                Kubernetes manifests (kustomize)
hack/                  Developer scripts
test/                  Integration & E2E tests
```

## Development

```bash
make generate    # Generate DeepCopy methods
make manifests   # Generate CRD YAML + RBAC
make test        # Run unit tests
make build       # Build the binary
make docker-build # Build the container image
```

## License

Apache 2.0
