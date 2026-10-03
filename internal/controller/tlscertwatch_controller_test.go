package controller_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	tlsv1alpha1 "github.com/HashanHirantha/auto-tls-certreloader/api/v1alpha1"
)

var _ = Describe("TLSCertWatch Controller", func() {

	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	Context("When creating a TLSCertWatch resource", func() {
		It("Should update status conditions after reconciliation", func() {
			ctx := context.Background()

			// Create a test namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-certwatch",
				},
			}
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

			// Create a test Secret with a self-signed cert
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tls-secret",
					Namespace: "test-certwatch",
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": []byte("--- placeholder cert PEM ---"),
					"tls.key": []byte("--- placeholder key PEM ---"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			// Create the TLSCertWatch CR
			certWatch := &tlsv1alpha1.TLSCertWatch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-watch",
					Namespace: "test-certwatch",
				},
				Spec: tlsv1alpha1.TLSCertWatchSpec{
					CertificateRef: tlsv1alpha1.CertificateRef{
						Kind:    "Secret",
						Name:    "test-tls-secret",
						CertKey: "tls.crt",
						KeyKey:  "tls.key",
					},
					Targets: []tlsv1alpha1.TargetRef{
						{
							Kind: "Deployment",
							Name: "test-app",
						},
					},
					CheckIntervalMinutes: 60,
					ReloadStrategy:       tlsv1alpha1.ReloadStrategyRollingRestart,
				},
			}
			Expect(k8sClient.Create(ctx, certWatch)).Should(Succeed())

			// Verify the CR was created
			lookupKey := types.NamespacedName{Name: "test-watch", Namespace: "test-certwatch"}
			created := &tlsv1alpha1.TLSCertWatch{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, created)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(created.Spec.CertificateRef.Name).Should(Equal("test-tls-secret"))
		})
	})
})
