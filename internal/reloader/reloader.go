// Package reloader implements workload restart strategies for the certreloader operator.
package reloader

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AnnotationKey is the pod template annotation used to trigger rolling restarts.
	AnnotationKey = "certreloader.io/restartedAt"
)

// Strategy defines the interface for workload reload strategies.
type Strategy interface {
	// Reload triggers a reload/restart of the specified workload.
	Reload(ctx context.Context, kind, name, namespace string) error
}

// RollingRestartStrategy implements Strategy by patching the pod template
// annotation, causing Kubernetes to perform a rolling update.
type RollingRestartStrategy struct {
	Client client.Client
}

// NewRollingRestartStrategy creates a new RollingRestartStrategy.
func NewRollingRestartStrategy(c client.Client) *RollingRestartStrategy {
	return &RollingRestartStrategy{Client: c}
}

// Reload patches the target workload's pod template annotation to trigger a rolling restart.
func (s *RollingRestartStrategy) Reload(ctx context.Context, kind, name, namespace string) error {
	patchData := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"%s":"%s"}}}}}`,
		AnnotationKey,
		time.Now().Format(time.RFC3339),
	))

	key := types.NamespacedName{Name: name, Namespace: namespace}

	switch kind {
	case "Deployment":
		var obj appsv1.Deployment
		if err := s.Client.Get(ctx, key, &obj); err != nil {
			return fmt.Errorf("get Deployment %s/%s: %w", namespace, name, err)
		}
		return s.Client.Patch(ctx, &obj, client.RawPatch(types.StrategicMergePatchType, patchData))

	case "StatefulSet":
		var obj appsv1.StatefulSet
		if err := s.Client.Get(ctx, key, &obj); err != nil {
			return fmt.Errorf("get StatefulSet %s/%s: %w", namespace, name, err)
		}
		return s.Client.Patch(ctx, &obj, client.RawPatch(types.StrategicMergePatchType, patchData))

	case "DaemonSet":
		var obj appsv1.DaemonSet
		if err := s.Client.Get(ctx, key, &obj); err != nil {
			return fmt.Errorf("get DaemonSet %s/%s: %w", namespace, name, err)
		}
		return s.Client.Patch(ctx, &obj, client.RawPatch(types.StrategicMergePatchType, patchData))

	default:
		return fmt.Errorf("unsupported workload kind: %s", kind)
	}
}

// AnnotationOnlyStrategy implements Strategy by updating only the CR annotation
// without forcing a workload restart. Useful for tracking cert changes.
type AnnotationOnlyStrategy struct {
	Client client.Client
}

// NewAnnotationOnlyStrategy creates a new AnnotationOnlyStrategy.
func NewAnnotationOnlyStrategy(c client.Client) *AnnotationOnlyStrategy {
	return &AnnotationOnlyStrategy{Client: c}
}

// Reload updates the workload's metadata annotation (not the pod template),
// so the workload is aware of the cert change but does not restart.
func (s *AnnotationOnlyStrategy) Reload(ctx context.Context, kind, name, namespace string) error {
	patchData := []byte(fmt.Sprintf(
		`{"metadata":{"annotations":{"%s":"%s"}}}`,
		AnnotationKey,
		time.Now().Format(time.RFC3339),
	))

	key := types.NamespacedName{Name: name, Namespace: namespace}

	switch kind {
	case "Deployment":
		var obj appsv1.Deployment
		if err := s.Client.Get(ctx, key, &obj); err != nil {
			return fmt.Errorf("get Deployment %s/%s: %w", namespace, name, err)
		}
		return s.Client.Patch(ctx, &obj, client.RawPatch(types.MergePatchType, patchData))

	case "StatefulSet":
		var obj appsv1.StatefulSet
		if err := s.Client.Get(ctx, key, &obj); err != nil {
			return fmt.Errorf("get StatefulSet %s/%s: %w", namespace, name, err)
		}
		return s.Client.Patch(ctx, &obj, client.RawPatch(types.MergePatchType, patchData))

	case "DaemonSet":
		var obj appsv1.DaemonSet
		if err := s.Client.Get(ctx, key, &obj); err != nil {
			return fmt.Errorf("get DaemonSet %s/%s: %w", namespace, name, err)
		}
		return s.Client.Patch(ctx, &obj, client.RawPatch(types.MergePatchType, patchData))

	default:
		return fmt.Errorf("unsupported workload kind: %s", kind)
	}
}
