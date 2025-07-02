// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
)

var _ = Describe("ExportPolicy Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		exportpolicy := &telemetryv1alpha1.ExportPolicy{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ExportPolicy")
			err := k8sClient.Get(ctx, typeNamespacedName, exportpolicy)
			if err != nil && errors.IsNotFound(err) {
				resource := &telemetryv1alpha1.ExportPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: telemetryv1alpha1.ExportPolicySpec{
						Sources: []telemetryv1alpha1.TelemetrySource{
							{
								Name: "metrics",
								Metrics: &telemetryv1alpha1.MetricSource{
									MetricsQL: `{service_name="gateway.networking.k8s.io", resource_type="gateways"}`,
								},
							},
						},
						Sinks: []telemetryv1alpha1.TelemetrySink{
							{
								Name:    "grafana-cloud",
								Sources: []string{"metrics"},
								Target: &telemetryv1alpha1.SinkTarget{
									PrometheusRemoteWrite: &telemetryv1alpha1.PrometheusRemoteWriteSink{
										Endpoint: "https://otlp-gateway-prod-eu-west-0.grafana.net/otlp",
										Authentication: &telemetryv1alpha1.Authentication{
											BasicAuth: &telemetryv1alpha1.BasicAuthAuthentication{
												SecretRef: telemetryv1alpha1.LocalSecretReference{
													Name: "grafana-push-api-token",
												},
											},
										},
										Batch: telemetryv1alpha1.Batch{
											Timeout: metav1.Duration{Duration: 5 * time.Second},
											MaxSize: 500,
										},
										Retry: telemetryv1alpha1.Retry{
											MaxAttempts:     3,
											BackoffDuration: metav1.Duration{Duration: 5 * time.Second},
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &telemetryv1alpha1.ExportPolicy{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ExportPolicy")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			finalizers := finalizer.NewFinalizers()
			secretFinalizer := &vectorSecretFinalizer{
				downstreamClient:                k8sClient,
				downstreamVectorConfigNamespace: "default",
			}
			Expect(finalizers.Register(exportPolicyControllerFinalizer, secretFinalizer)).To(Succeed())

			controllerReconciler := &ExportPolicyReconciler{
				VectorConfigLabelKey:            "telemetry.miloapis.com/vector-export-policy-config",
				VectorConfigLabelValue:          "true",
				mgr:                             mgr,
				DownstreamClient:                k8sClient,
				DownstreamVectorConfigNamespace: "default",
				finalizers:                      finalizers,
			}

			_, err := controllerReconciler.Reconcile(ctx, mcreconcile.Request{
				ClusterName: "test-cluster",
				Request:     ctrl.Request{NamespacedName: typeNamespacedName},
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
