package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertificateRef identifies the Secret or ConfigMap containing TLS certificate data.
type CertificateRef struct {
	// Kind is the Kubernetes resource type: "Secret" or "ConfigMap".
	// +kubebuilder:validation:Enum=Secret;ConfigMap
	// +kubebuilder:default=Secret
	Kind string `json:"kind"`

	// Name of the Secret or ConfigMap.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace of the Secret or ConfigMap. Defaults to the CR namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// CertKey is the key within the Secret/ConfigMap holding the certificate PEM.
	// +kubebuilder:default="tls.crt"
	CertKey string `json:"certKey,omitempty"`

	// KeyKey is the key within the Secret/ConfigMap holding the private key PEM.
	// +kubebuilder:default="tls.key"
	KeyKey string `json:"keyKey,omitempty"`

	// CAKey is the optional key holding the CA bundle PEM.
	// +optional
	CAKey string `json:"caKey,omitempty"`
}

// ValidationPolicy configures how certificates are validated.
type ValidationPolicy struct {
	// CheckExpiry enables expiry threshold checking.
	// +kubebuilder:default=true
	CheckExpiry bool `json:"checkExpiry,omitempty"`

	// ExpiryThresholdDays is the number of days before expiry to trigger a warning/reload.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	ExpiryThresholdDays int `json:"expiryThresholdDays,omitempty"`

	// CheckChain enables full certificate chain verification.
	// +kubebuilder:default=true
	CheckChain bool `json:"checkChain,omitempty"`

	// CheckHostnames is an optional list of hostnames to verify against the cert SANs/CN.
	// +optional
	CheckHostnames []string `json:"checkHostnames,omitempty"`
}

// TargetRef identifies a workload to reload when certificate changes are detected.
type TargetRef struct {
	// Kind of the target workload.
	// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet
	Kind string `json:"kind"`

	// Name of the target workload.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace of the target workload. Defaults to the CR namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ReloadStrategy defines how the controller triggers workload restarts.
// +kubebuilder:validation:Enum=RollingRestart;Annotation
type ReloadStrategy string

const (
	// ReloadStrategyRollingRestart patches the pod template annotation to trigger a rollout.
	ReloadStrategyRollingRestart ReloadStrategy = "RollingRestart"

	// ReloadStrategyAnnotation only updates an annotation without forcing a restart.
	ReloadStrategyAnnotation ReloadStrategy = "Annotation"
)

// TLSCertWatchSpec defines the desired state of TLSCertWatch.
type TLSCertWatchSpec struct {
	// CertificateRef points to the Secret or ConfigMap containing the TLS certificate.
	CertificateRef CertificateRef `json:"certificateRef"`

	// Validation configures certificate validation behavior.
	// +optional
	Validation ValidationPolicy `json:"validation,omitempty"`

	// Targets lists the workloads to reload when the certificate changes or becomes invalid.
	// +kubebuilder:validation:MinItems=1
	Targets []TargetRef `json:"targets"`

	// CheckIntervalMinutes defines how often (in minutes) to re-validate the certificate
	// in addition to watch-based triggers.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=60
	CheckIntervalMinutes int `json:"checkIntervalMinutes,omitempty"`

	// ReloadStrategy defines how workload restarts are triggered.
	// +kubebuilder:default=RollingRestart
	ReloadStrategy ReloadStrategy `json:"reloadStrategy,omitempty"`

	// Paused suspends reconciliation when set to true.
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// TargetReloadStatus tracks the reload state of a single target workload.
type TargetReloadStatus struct {
	// Kind of the target workload.
	Kind string `json:"kind"`

	// Name of the target workload.
	Name string `json:"name"`

	// Namespace of the target workload.
	Namespace string `json:"namespace"`

	// LastReloadedAt is the timestamp of the last successful reload.
	// +optional
	LastReloadedAt *metav1.Time `json:"lastReloadedAt,omitempty"`

	// ReloadedForFingerprint is the certificate fingerprint that triggered the last reload.
	// +optional
	ReloadedForFingerprint string `json:"reloadedForFingerprint,omitempty"`

	// UpToDate indicates whether this target has been reloaded for the current certificate.
	UpToDate bool `json:"upToDate"`
}

// TLSCertWatchStatus defines the observed state of TLSCertWatch.
type TLSCertWatchStatus struct {
	// Conditions represent the latest available observations of the resource's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastCheckedAt is the timestamp of the last successful validation check.
	// +optional
	LastCheckedAt *metav1.Time `json:"lastCheckedAt,omitempty"`

	// CertificateExpiry is the expiration time of the certificate (x509 NotAfter).
	// +optional
	CertificateExpiry *metav1.Time `json:"certificateExpiry,omitempty"`

	// CertificateFingerprint is the SHA-256 fingerprint of the certificate DER bytes.
	// +optional
	CertificateFingerprint string `json:"certificateFingerprint,omitempty"`

	// CertificateSubject is the Subject Common Name from the certificate.
	// +optional
	CertificateSubject string `json:"certificateSubject,omitempty"`

	// CertificateSANs lists the Subject Alternative Names from the certificate.
	// +optional
	CertificateSANs []string `json:"certificateSANs,omitempty"`

	// TargetStatuses tracks per-target reload state.
	// +optional
	TargetStatuses []TargetReloadStatus `json:"targetStatuses,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tcw
// +kubebuilder:printcolumn:name="Certificate",type=string,JSONPath=`.status.certificateSubject`
// +kubebuilder:printcolumn:name="Valid",type=string,JSONPath=`.status.conditions[?(@.type=="CertificateValid")].status`
// +kubebuilder:printcolumn:name="Expires",type=date,JSONPath=`.status.certificateExpiry`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TLSCertWatch is the Schema for the tlscertwatches API.
// It watches a TLS certificate stored in a Secret or ConfigMap and triggers
// rolling restarts of target workloads when the certificate changes or becomes invalid.
type TLSCertWatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TLSCertWatchSpec   `json:"spec,omitempty"`
	Status TLSCertWatchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TLSCertWatchList contains a list of TLSCertWatch resources.
type TLSCertWatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TLSCertWatch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TLSCertWatch{}, &TLSCertWatchList{})
}
