// Package controller implements the reconciliation loop for TLSCertWatch resources.
package controller

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tlsv1alpha1 "github.com/HashanHirantha/auto-tls-certreloader/api/v1alpha1"
)

const (
	restartAnnotationKey = "certreloader.io/restartedAt"
)

// TLSCertWatchReconciler reconciles a TLSCertWatch object.
type TLSCertWatchReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=tls.certreloader.io,resources=tlscertwatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tls.certreloader.io,resources=tlscertwatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tls.certreloader.io,resources=tlscertwatches/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets,verbs=get;list;watch;update;patch

// Reconcile is the main reconciliation loop for TLSCertWatch resources.
// It fetches the referenced certificate, validates it, and triggers workload
// reloads if the certificate has changed or is invalid.
func (r *TLSCertWatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the TLSCertWatch CR
	var certWatch tlsv1alpha1.TLSCertWatch
	if err := r.Get(ctx, req.NamespacedName, &certWatch); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("TLSCertWatch resource not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Check if reconciliation is paused
	if certWatch.Spec.Paused {
		logger.Info("TLSCertWatch is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// 3. Resolve the certificate namespace
	certNamespace := certWatch.Spec.CertificateRef.Namespace
	if certNamespace == "" {
		certNamespace = certWatch.Namespace
	}

	// 4. Fetch the certificate data from Secret or ConfigMap
	certPEM, err := r.fetchCertificateData(ctx, &certWatch, certNamespace)
	if err != nil {
		r.Recorder.Eventf(&certWatch, corev1.EventTypeWarning, "SecretNotFound",
			"Failed to fetch certificate data: %v", err)
		r.setCondition(&certWatch, "CertificateValid", metav1.ConditionFalse,
			"FetchFailed", fmt.Sprintf("Failed to fetch certificate data: %v", err))
		_ = r.Status().Update(ctx, &certWatch)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	// 5. Parse and validate the certificate
	cert, validationErr := r.validateCertificate(certPEM, &certWatch)

	// 6. Compute fingerprint
	var fingerprint string
	if cert != nil {
		hash := sha256.Sum256(cert.Raw)
		fingerprint = hex.EncodeToString(hash[:])
	}

	// 7. Update status with certificate metadata
	now := metav1.Now()
	certWatch.Status.LastCheckedAt = &now
	certWatch.Status.ObservedGeneration = certWatch.Generation

	if cert != nil {
		expiry := metav1.NewTime(cert.NotAfter)
		certWatch.Status.CertificateExpiry = &expiry
		certWatch.Status.CertificateSubject = cert.Subject.CommonName
		certWatch.Status.CertificateFingerprint = fingerprint

		sans := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses))
		sans = append(sans, cert.DNSNames...)
		for _, ip := range cert.IPAddresses {
			sans = append(sans, ip.String())
		}
		certWatch.Status.CertificateSANs = sans
	}

	if validationErr != nil {
		r.Recorder.Eventf(&certWatch, corev1.EventTypeWarning, "CertificateInvalid",
			"Certificate validation failed: %v", validationErr)
		r.setCondition(&certWatch, "CertificateValid", metav1.ConditionFalse,
			"ValidationFailed", validationErr.Error())
	} else {
		// Check expiry threshold
		threshold := time.Duration(certWatch.Spec.Validation.ExpiryThresholdDays) * 24 * time.Hour
		if certWatch.Spec.Validation.CheckExpiry && time.Until(cert.NotAfter) < threshold {
			daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
			r.Recorder.Eventf(&certWatch, corev1.EventTypeWarning, "CertificateExpiring",
				"Certificate expires in %d days (threshold: %d days)", daysLeft,
				certWatch.Spec.Validation.ExpiryThresholdDays)
			r.setCondition(&certWatch, "CertificateExpiring", metav1.ConditionTrue,
				"ExpiringSoon", fmt.Sprintf("Certificate expires in %d days", daysLeft))
		} else {
			r.setCondition(&certWatch, "CertificateExpiring", metav1.ConditionFalse,
				"NotExpiring", "Certificate has sufficient lifetime")
		}

		r.setCondition(&certWatch, "CertificateValid", metav1.ConditionTrue,
			"Valid", "Certificate is valid")
		r.Recorder.Event(&certWatch, corev1.EventTypeNormal, "CertificateValid",
			"Certificate is valid")
	}

	// 8. Check if reload is needed (fingerprint changed)
	previousFingerprint := certWatch.Status.CertificateFingerprint
	needsReload := fingerprint != "" && fingerprint != previousFingerprint && previousFingerprint != ""

	if needsReload {
		r.setCondition(&certWatch, "ReloadRequired", metav1.ConditionTrue,
			"FingerprintChanged", "Certificate fingerprint changed, reload required")

		// 9. Trigger reload on all targets
		for i, target := range certWatch.Spec.Targets {
			targetNS := target.Namespace
			if targetNS == "" {
				targetNS = certWatch.Namespace
			}

			if err := r.reloadTarget(ctx, target.Kind, target.Name, targetNS); err != nil {
				r.Recorder.Eventf(&certWatch, corev1.EventTypeWarning, "ReloadFailed",
					"Failed to reload %s/%s: %v", target.Kind, target.Name, err)
				logger.Error(err, "failed to reload target",
					"kind", target.Kind, "name", target.Name)
			} else {
				r.Recorder.Eventf(&certWatch, corev1.EventTypeNormal, "ReloadTriggered",
					"Rolling restart triggered for %s/%s", target.Kind, target.Name)

				// Update per-target status
				reloadTime := metav1.Now()
				status := tlsv1alpha1.TargetReloadStatus{
					Kind:                   target.Kind,
					Name:                   target.Name,
					Namespace:              targetNS,
					LastReloadedAt:         &reloadTime,
					ReloadedForFingerprint: fingerprint,
					UpToDate:               true,
				}
				if i < len(certWatch.Status.TargetStatuses) {
					certWatch.Status.TargetStatuses[i] = status
				} else {
					certWatch.Status.TargetStatuses = append(certWatch.Status.TargetStatuses, status)
				}
			}
		}

		r.setCondition(&certWatch, "ReloadRequired", metav1.ConditionFalse,
			"ReloadComplete", "All targets have been reloaded")
	}

	// 10. Persist status
	if err := r.Status().Update(ctx, &certWatch); err != nil {
		logger.Error(err, "failed to update TLSCertWatch status")
		return ctrl.Result{}, err
	}

	// 11. Requeue after the configured check interval
	requeueAfter := time.Duration(certWatch.Spec.CheckIntervalMinutes) * time.Minute
	logger.Info("reconciliation complete", "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// fetchCertificateData retrieves the certificate PEM data from the referenced Secret or ConfigMap.
func (r *TLSCertWatchReconciler) fetchCertificateData(
	ctx context.Context,
	certWatch *tlsv1alpha1.TLSCertWatch,
	namespace string,
) ([]byte, error) {
	ref := certWatch.Spec.CertificateRef
	key := types.NamespacedName{Name: ref.Name, Namespace: namespace}

	switch ref.Kind {
	case "Secret":
		var secret corev1.Secret
		if err := r.Get(ctx, key, &secret); err != nil {
			return nil, fmt.Errorf("failed to get Secret %s/%s: %w", namespace, ref.Name, err)
		}
		certData, ok := secret.Data[ref.CertKey]
		if !ok {
			return nil, fmt.Errorf("key %q not found in Secret %s/%s", ref.CertKey, namespace, ref.Name)
		}
		return certData, nil

	case "ConfigMap":
		var cm corev1.ConfigMap
		if err := r.Get(ctx, key, &cm); err != nil {
			return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, ref.Name, err)
		}
		certData, ok := cm.Data[ref.CertKey]
		if !ok {
			return nil, fmt.Errorf("key %q not found in ConfigMap %s/%s", ref.CertKey, namespace, ref.Name)
		}
		return []byte(certData), nil

	default:
		return nil, fmt.Errorf("unsupported certificateRef kind: %s", ref.Kind)
	}
}

// validateCertificate parses and validates a PEM-encoded X.509 certificate.
func (r *TLSCertWatchReconciler) validateCertificate(
	certPEM []byte,
	certWatch *tlsv1alpha1.TLSCertWatch,
) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X.509 certificate: %w", err)
	}

	// Check expiry
	if certWatch.Spec.Validation.CheckExpiry {
		if time.Now().After(cert.NotAfter) {
			return cert, fmt.Errorf("certificate has expired at %s", cert.NotAfter.Format(time.RFC3339))
		}
	}

	// Check hostnames
	for _, hostname := range certWatch.Spec.Validation.CheckHostnames {
		if err := cert.VerifyHostname(hostname); err != nil {
			return cert, fmt.Errorf("hostname verification failed for %q: %w", hostname, err)
		}
	}

	// Check chain validity (self-signed certs will fail here — this is expected)
	if certWatch.Spec.Validation.CheckChain {
		pool := x509.NewCertPool()
		pool.AddCert(cert) // For self-signed, add the cert itself as a root
		opts := x509.VerifyOptions{
			Roots: pool,
		}
		if _, err := cert.Verify(opts); err != nil {
			return cert, fmt.Errorf("certificate chain verification failed: %w", err)
		}
	}

	return cert, nil
}

// reloadTarget triggers a rolling restart of the specified workload by patching
// the pod template annotation.
func (r *TLSCertWatchReconciler) reloadTarget(
	ctx context.Context,
	kind, name, namespace string,
) error {
	patchData := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"%s":"%s"}}}}}`,
		restartAnnotationKey,
		time.Now().Format(time.RFC3339),
	))

	key := types.NamespacedName{Name: name, Namespace: namespace}

	switch kind {
	case "Deployment":
		var deploy appsv1.Deployment
		if err := r.Get(ctx, key, &deploy); err != nil {
			return fmt.Errorf("failed to get Deployment %s/%s: %w", namespace, name, err)
		}
		return r.Patch(ctx, &deploy, client.RawPatch(types.StrategicMergePatchType, patchData))

	case "StatefulSet":
		var sts appsv1.StatefulSet
		if err := r.Get(ctx, key, &sts); err != nil {
			return fmt.Errorf("failed to get StatefulSet %s/%s: %w", namespace, name, err)
		}
		return r.Patch(ctx, &sts, client.RawPatch(types.StrategicMergePatchType, patchData))

	case "DaemonSet":
		var ds appsv1.DaemonSet
		if err := r.Get(ctx, key, &ds); err != nil {
			return fmt.Errorf("failed to get DaemonSet %s/%s: %w", namespace, name, err)
		}
		return r.Patch(ctx, &ds, client.RawPatch(types.StrategicMergePatchType, patchData))

	default:
		return fmt.Errorf("unsupported target kind: %s", kind)
	}
}

// setCondition updates or appends a condition in the TLSCertWatch status.
func (r *TLSCertWatchReconciler) setCondition(
	certWatch *tlsv1alpha1.TLSCertWatch,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	now := metav1.Now()
	for i, c := range certWatch.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				certWatch.Status.Conditions[i].LastTransitionTime = now
			}
			certWatch.Status.Conditions[i].Status = status
			certWatch.Status.Conditions[i].Reason = reason
			certWatch.Status.Conditions[i].Message = message
			certWatch.Status.Conditions[i].ObservedGeneration = certWatch.Generation
			return
		}
	}
	certWatch.Status.Conditions = append(certWatch.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: certWatch.Generation,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *TLSCertWatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tlsv1alpha1.TLSCertWatch{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
