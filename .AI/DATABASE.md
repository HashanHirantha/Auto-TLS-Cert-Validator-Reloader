# Database & Storage Design

This operator is **stateless by design** — all persistent state lives in the Kubernetes API
server (etcd) as Custom Resource status fields. This document covers every data model and
storage mechanism used by the operator.

---

## 1. Primary Data Store: Kubernetes etcd (via CRD Status)

The operator follows the Kubernetes **level-triggered reconciliation** model:

- **Desired state** is expressed in the CRD `.spec` (written by the user).
- **Observed state** is recorded in the CRD `.status` (written by the controller).
- The reconciler continuously drives observed → desired.

There is **no external database** (no SQL, no Redis, no S3). This is intentional —
operators should be as dependency-free as possible.

### 1.1 TLSCertWatch Status Schema

```go
// TLSCertWatchStatus defines the observed state of TLSCertWatch
type TLSCertWatchStatus struct {
    // Standard Kubernetes conditions
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // Timestamp of the last successful validation check
    LastCheckedAt *metav1.Time `json:"lastCheckedAt,omitempty"`

    // When the certificate expires (parsed from x509 NotAfter)
    CertificateExpiry *metav1.Time `json:"certificateExpiry,omitempty"`

    // SHA-256 fingerprint of the current certificate DER bytes
    CertificateFingerprint string `json:"certificateFingerprint,omitempty"`

    // Subject Common Name extracted from the certificate
    CertificateSubject string `json:"certificateSubject,omitempty"`

    // SANs (Subject Alternative Names) extracted from the certificate
    CertificateSANs []string `json:"certificateSANs,omitempty"`

    // Per-target reload status
    TargetStatuses []TargetReloadStatus `json:"targetStatuses,omitempty"`

    // Last generation observed by the controller
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// TargetReloadStatus tracks the reload state of a single target workload
type TargetReloadStatus struct {
    Kind      string       `json:"kind"`
    Name      string       `json:"name"`
    Namespace string       `json:"namespace"`
    LastReloadedAt *metav1.Time `json:"lastReloadedAt,omitempty"`
    // Fingerprint of the cert that was active at last reload
    ReloadedForFingerprint string `json:"reloadedForFingerprint,omitempty"`
    // Whether this target is currently up-to-date
    UpToDate bool `json:"upToDate"`
}
```

### 1.2 Condition Types

| Condition Type       | True Meaning                        | False Meaning                       |
| -------------------- | ----------------------------------- | ----------------------------------- |
| `CertificateValid`   | Cert parsed, chain valid, not expired | Parse error, chain invalid, or expired |
| `CertificateExpiring`| Cert expires within threshold       | Cert has sufficient lifetime        |
| `ReloadRequired`     | Fingerprint changed, targets need restart | All targets up-to-date         |
| `Ready`              | All checks pass, all targets current | Something needs attention          |

---

## 2. In-Memory Caches

### 2.1 Controller-Runtime Informer Cache

The controller-runtime `Manager` maintains an **in-memory informer cache** backed by
Kubernetes watches. The operator caches:

| Resource              | Reason                                                   |
| --------------------- | -------------------------------------------------------- |
| `TLSCertWatch`        | Primary resource — drives reconciliation                 |
| `Secret`              | Watched for cert data changes                            |
| `ConfigMap`           | Watched when `certificateRef.kind == ConfigMap`          |
| `Deployment`          | Target for rolling restarts                              |
| `StatefulSet`         | Target for rolling restarts                              |
| `DaemonSet`           | Target for rolling restarts                              |

Cache is **namespace-scoped** when the operator runs in single-namespace mode,
or **cluster-scoped** when `--watch-all-namespaces` is set.

### 2.2 Fingerprint Cache (Optimization)

A lightweight in-process `sync.Map` caches the last-seen fingerprint per
`namespace/name` key to avoid redundant cert parsing when the Secret hasn't changed:

```go
// internal/cache/fingerprint.go
type FingerprintCache struct {
    store sync.Map // key: "namespace/secretName" → value: string (sha256 hex)
}

func (c *FingerprintCache) HasChanged(key, newFingerprint string) bool {
    old, loaded := c.store.LoadOrStore(key, newFingerprint)
    if !loaded {
        return true // first time seeing this key
    }
    if old.(string) != newFingerprint {
        c.store.Store(key, newFingerprint)
        return true
    }
    return false
}
```

> **Note:** This cache is ephemeral — it is lost on operator restart. The authoritative
> fingerprint is always the one stored in `status.certificateFingerprint`.

---

## 3. Prometheus Metrics Store

Metrics are held in-memory by the Prometheus client library and scraped via HTTP.
No disk persistence.

```go
var (
    certExpirySeconds = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "certreloader_certificate_expiry_seconds",
            Help: "Seconds until the watched certificate expires",
        },
        []string{"namespace", "name", "secret"},
    )
    certValid = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "certreloader_certificate_valid",
            Help: "1 if the certificate is valid, 0 otherwise",
        },
        []string{"namespace", "name", "secret"},
    )
    reloadsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "certreloader_reloads_total",
            Help: "Total number of workload reloads triggered",
        },
        []string{"namespace", "name", "target_kind", "target_name"},
    )
)
```

---

## 4. Kubernetes Events (Audit Trail)

Events are short-lived records stored in etcd (default TTL: 1 hour). They serve as an
audit log visible via `kubectl describe tlscertwatch <name>`.

| Event Type | Reason              | Message Example                              |
| ---------- | ------------------- | -------------------------------------------- |
| Normal     | CertificateValid    | "Certificate valid, expires in 90d"          |
| Warning    | CertificateExpiring | "Certificate expires in 15d (threshold: 30d)"|
| Warning    | CertificateInvalid  | "x509: certificate has expired"              |
| Normal     | ReloadTriggered     | "Rolling restart triggered for Deployment/my-app" |
| Warning    | ReloadFailed        | "Failed to patch Deployment/my-app: forbidden" |
| Warning    | SecretNotFound      | "Referenced Secret 'my-tls' not found"       |

---

## 5. Data Flow Diagram

```
User creates/updates          Kubernetes API Server (etcd)
TLSCertWatch CR ──────────►  ┌────────────────────────────┐
                              │  TLSCertWatch Spec & Status │
                              │  Secret (TLS data)          │
                              │  Deployment/SS/DS           │
                              └──────────┬─────────────────┘
                                         │ watch stream
                                         ▼
                              ┌────────────────────────────┐
                              │  Controller (in-memory)     │
                              │  ├── Informer Cache         │
                              │  ├── Fingerprint Cache      │
                              │  └── Prometheus Metrics     │
                              └──────────┬─────────────────┘
                                         │ status update / patch
                                         ▼
                              ┌────────────────────────────┐
                              │  etcd (via API server)      │
                              │  ├── .status update         │
                              │  ├── Event records          │
                              │  └── Deployment annotation  │
                              └────────────────────────────┘
```

---

## 6. Backup & Disaster Recovery

Since all state is in etcd:

- **etcd snapshots** capture the full operator state (CRDs, CRs, Secrets).
- The operator itself is **stateless** — it can be deleted and redeployed with zero data loss.
- On startup, the reconciler re-validates every TLSCertWatch CR and converges to the
  correct state automatically (level-triggered, not edge-triggered).

---

## 7. Why No External Database?

| Concern               | How We Handle It Without a DB                   |
| ---------------------- | ------------------------------------------------ |
| Persistent state       | CRD `.status` in etcd                            |
| Historical audit       | Kubernetes Events + Prometheus metrics + logging |
| Leader election        | `controller-runtime` LeaderElection (etcd lease) |
| Configuration          | Operator flags + ConfigMap                       |
| Cert metadata cache    | In-memory `sync.Map` (non-critical, ephemeral)  |

Adding an external database would violate the **operator pattern** principle that the
Kubernetes API server is the single source of truth.
