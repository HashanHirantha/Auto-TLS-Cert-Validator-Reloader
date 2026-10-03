// Package metrics defines Prometheus metrics for the certreloader operator.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// CertExpirySeconds tracks the time until certificate expiry in seconds.
	CertExpirySeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certreloader_certificate_expiry_seconds",
			Help: "Seconds until the watched certificate expires.",
		},
		[]string{"namespace", "name", "secret"},
	)

	// CertValid indicates whether the watched certificate is valid (1) or not (0).
	CertValid = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certreloader_certificate_valid",
			Help: "1 if the certificate is valid, 0 otherwise.",
		},
		[]string{"namespace", "name", "secret"},
	)

	// ReloadsTotal counts the total number of workload reloads triggered.
	ReloadsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "certreloader_reloads_total",
			Help: "Total number of workload reloads triggered by cert changes.",
		},
		[]string{"namespace", "name", "target_kind", "target_name"},
	)

	// ReconcileErrorsTotal counts the total number of reconciliation errors.
	ReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "certreloader_reconcile_errors_total",
			Help: "Total number of reconciliation errors.",
		},
		[]string{"namespace", "name"},
	)

	// ReconcileDurationSeconds tracks reconciliation loop duration.
	ReconcileDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "certreloader_reconcile_duration_seconds",
			Help:    "Duration of reconciliation loops in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	// Register all metrics with the controller-runtime metrics registry.
	metrics.Registry.MustRegister(
		CertExpirySeconds,
		CertValid,
		ReloadsTotal,
		ReconcileErrorsTotal,
		ReconcileDurationSeconds,
	)
}
