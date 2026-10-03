# Auto-TLS Cert Validator & Reloader — Project Overview

## Vision

A **Custom Kubernetes Operator** written in Go that automatically validates TLS certificates
mounted in cluster workloads, detects expiring or invalid certs, and triggers rolling restarts
of affected Pods — all driven by a declarative Custom Resource Definition (CRD).

---

## Problem Statement

Managing TLS certificates in Kubernetes is operationally painful:

1. Certificates expire silently, causing outages.
2. After cert renewal (e.g., via cert-manager or Vault), Pods keep serving the **old** cert
   until they are manually restarted.
3. There is no built-in Kubernetes primitive that watches a Secret/ConfigMap for cert changes
   and automatically rolls the consuming Deployments.

This operator closes that gap.

---

## High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                           │
│                                                                  │
│  ┌─────────────┐     watches      ┌──────────────────────┐      │
│  │  CertReloader│ ◄────────────── │  TLSCertWatch CRD    │      │
│  │  Controller  │                 │  (user-defined spec)  │      │
│  └──────┬───────┘                 └──────────────────────┘      │
│         │                                                        │
│         │ reconcile loop                                         │
│         ▼                                                        │
│  ┌──────────────┐   reads cert    ┌──────────────────────┐      │
│  │  Cert        │ ◄────────────── │  Secret / ConfigMap  │      │
│  │  Validator   │                 │  (TLS data)          │      │
│  └──────┬───────┘                 └──────────────────────┘      │
│         │                                                        │
│         │ if invalid / expiring / changed                        │
│         ▼                                                        │
│  ┌──────────────┐   patch/restart ┌──────────────────────┐      │
│  │  Reloader    │ ───────────────►│  Deployment / SS /   │      │
│  │  Engine      │                 │  DaemonSet Pods      │      │
│  └──────────────┘                 └──────────────────────┘      │
│                                                                  │
│  ┌──────────────┐                                                │
│  │  Metrics     │  ── Prometheus /metrics endpoint               │
│  │  & Events    │  ── Kubernetes Events on CRD status            │
│  └──────────────┘                                                │
└──────────────────────────────────────────────────────────────────┘
```

---

## Core Tech Stack

| Layer              | Technology                              |
| ------------------ | --------------------------------------- |
| Language           | Go 1.22+                                |
| Operator Framework | Kubebuilder v4 / controller-runtime     |
| K8s Client         | client-go, apimachinery                 |
| CRD Code-Gen       | controller-gen (markers → YAML + deepcopy) |
| Testing            | envtest, Ginkgo/Gomega, testify         |
| Local Cluster      | kind (Kubernetes in Docker) or minikube |
| Container          | Docker / ko                             |
| CI                 | GitHub Actions                          |
| Metrics            | Prometheus client_golang                |

---

## Target Users

- Platform / SRE teams who manage many TLS-terminating services.
- Developers running cert-manager who want zero-touch Pod restarts after renewal.
- Anyone learning how to build production-grade Kubernetes operators in Go.

---

## Project Status

| Milestone            | Status      |
| -------------------- | ----------- |
| CRD Design           | 🔲 Planned |
| Reconciler Core      | 🔲 Planned |
| Cert Validation      | 🔲 Planned |
| Rolling Restart       | 🔲 Planned |
| Prometheus Metrics   | 🔲 Planned |
| Helm Chart           | 🔲 Planned |
| E2E Tests            | 🔲 Planned |
| CI/CD Pipeline       | 🔲 Planned |

---

## Repository Layout

See `FILE_STRUCTURE.md` in this folder for the full directory tree.
