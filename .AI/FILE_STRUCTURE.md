# File Structure — Auto-TLS Cert Validator & Reloader

Complete directory layout for the Kubernetes operator project.

```
auto-tls-certreloader/
│
├── .AI/                                    # AI project documentation
│   ├── PROJECT_OVERVIEW.md                 # Vision, architecture, tech stack
│   ├── FEATURES.md                         # Feature tracking & implementation plan
│   ├── DATABASE.md                         # Data models & storage design
│   └── FILE_STRUCTURE.md                   # This file
│
├── .github/                                # GitHub configuration
│   └── workflows/
│       ├── ci.yaml                         # Lint, test, build on PR
│       └── release.yaml                    # Build & push image on tag
│
├── api/                                    # CRD API types (kubebuilder convention)
│   └── v1alpha1/
│       ├── groupversion_info.go            # SchemeBuilder, GroupVersion const
│       ├── tlscertwatch_types.go           # TLSCertWatch Spec & Status structs
│       ├── tlscertwatch_webhook.go         # Admission webhook (future)
│       └── zz_generated.deepcopy.go        # Auto-generated DeepCopy methods
│
├── cmd/                                    # Entrypoint
│   └── main.go                             # Manager bootstrap, flag parsing
│
├── config/                                 # Kubebuilder / Kustomize manifests
│   ├── crd/
│   │   ├── bases/
│   │   │   └── tls.certreloader.io_tlscertwatches.yaml  # Generated CRD YAML
│   │   └── kustomization.yaml
│   ├── default/
│   │   ├── kustomization.yaml              # Aggregates all overlays
│   │   └── manager_auth_proxy_patch.yaml
│   ├── manager/
│   │   ├── kustomization.yaml
│   │   └── manager.yaml                    # Deployment manifest for the operator
│   ├── rbac/
│   │   ├── kustomization.yaml
│   │   ├── role.yaml                       # Generated ClusterRole
│   │   ├── role_binding.yaml
│   │   ├── leader_election_role.yaml
│   │   ├── leader_election_role_binding.yaml
│   │   └── service_account.yaml
│   ├── samples/
│   │   └── tls_v1alpha1_tlscertwatch.yaml  # Example CR for users
│   ├── certmanager/                        # cert-manager integration (optional)
│   │   └── kustomization.yaml
│   └── prometheus/
│       ├── kustomization.yaml
│       └── monitor.yaml                    # ServiceMonitor for Prometheus
│
├── deploy/                                 # Alternative deployment methods
│   └── helm/
│       └── certreloader/
│           ├── Chart.yaml
│           ├── values.yaml
│           ├── templates/
│           │   ├── _helpers.tpl
│           │   ├── deployment.yaml
│           │   ├── serviceaccount.yaml
│           │   ├── clusterrole.yaml
│           │   ├── clusterrolebinding.yaml
│           │   └── servicemonitor.yaml
│           └── crds/
│               └── tls.certreloader.io_tlscertwatches.yaml
│
├── internal/                               # Private implementation packages
│   ├── controller/
│   │   ├── tlscertwatch_controller.go      # Reconciler implementation
│   │   ├── tlscertwatch_controller_test.go # Unit tests (fake client)
│   │   └── suite_test.go                   # envtest suite setup
│   │
│   ├── cert/
│   │   ├── validator.go                    # X.509 parsing, chain validation, expiry
│   │   ├── validator_test.go               # Table-driven tests with generated certs
│   │   ├── fingerprint.go                  # SHA-256 fingerprint computation
│   │   └── testdata/                       # Test certificates
│   │       ├── valid.pem
│   │       ├── expired.pem
│   │       ├── self-signed.pem
│   │       └── chain-bundle.pem
│   │
│   ├── reloader/
│   │   ├── reloader.go                     # Annotation-patch rolling restart logic
│   │   ├── reloader_test.go
│   │   └── strategy.go                     # ReloadStrategy interface
│   │
│   ├── cache/
│   │   ├── fingerprint.go                  # In-memory fingerprint cache (sync.Map)
│   │   └── fingerprint_test.go
│   │
│   └── metrics/
│       ├── metrics.go                      # Prometheus metric definitions
│       └── recorder.go                     # Helper to record cert metrics
│
├── hack/                                   # Developer scripts
│   ├── boilerplate.go.txt                  # License header template
│   ├── generate-test-certs.sh              # Generate self-signed certs for testing
│   └── kind-config.yaml                    # Kind cluster config for local dev
│
├── test/                                   # Integration & E2E tests
│   ├── e2e/
│   │   ├── e2e_suite_test.go
│   │   └── certwatch_e2e_test.go           # Full lifecycle test in kind cluster
│   └── utils/
│       └── testhelpers.go                  # Shared test utilities
│
├── Dockerfile                              # Multi-stage build
├── Makefile                                # Build, test, generate, deploy targets
├── PROJECT                                 # Kubebuilder project metadata
├── go.mod                                  # Go module definition
├── go.sum                                  # Dependency checksums
├── .gitignore
├── .golangci.yml                           # Linter configuration
├── README.md                               # User-facing documentation
├── LICENSE                                 # Apache 2.0
└── CONTRIBUTING.md                         # Contribution guidelines
```

---

## Key Directories Explained

| Directory      | Purpose                                                     |
| -------------- | ----------------------------------------------------------- |
| `.AI/`         | AI-assisted project docs (overview, features, data models)  |
| `api/`         | Public API types — the CRD Go structs with kubebuilder markers |
| `cmd/`         | Operator entrypoint — boots the controller manager          |
| `config/`      | Kubernetes manifests generated by kubebuilder + kustomize   |
| `deploy/helm/` | Helm chart for production deployment                        |
| `internal/`    | Private packages — the core business logic                  |
| `hack/`        | Developer-only scripts and configs                          |
| `test/`        | Integration and end-to-end tests                            |

---

## Code Generation Commands

```bash
# Generate DeepCopy methods, CRD YAML, and RBAC manifests
make generate
make manifests

# Under the hood, these run:
# controller-gen object paths=./api/...
# controller-gen crd rbac:roleName=certreloader-role paths=./... output:crd:dir=config/crd/bases
```
