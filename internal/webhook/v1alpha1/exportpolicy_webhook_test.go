// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// TODO (user): Add any additional imports if needed
)

var _ = Describe("ExportPolicy Webhook", func() {
	var (
		obj       *telemetryv1alpha1.ExportPolicy
		oldObj    *telemetryv1alpha1.ExportPolicy
		validator ExportPolicyCustomValidator
	)

	BeforeEach(func() {
		obj = &telemetryv1alpha1.ExportPolicy{
			Spec: telemetryv1alpha1.ExportPolicySpec{
				Sinks: []telemetryv1alpha1.TelemetrySink{
					{
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
		oldObj = obj.DeepCopy()
		validator = ExportPolicyCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		// TODO (user): Add any setup logic common to all tests
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("When creating or updating ExportPolicy under Validating Webhook", func() {
		// TODO (user): Add logic for validating webhooks
		// Example:
		// It("Should deny creation if a required field is missing", func() {
		//     By("simulating an invalid creation scenario")
		//     obj.SomeRequiredField = ""
		//     Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		// })
		//
		// It("Should admit creation if all required fields are present", func() {
		//     By("simulating an invalid creation scenario")
		//     obj.SomeRequiredField = "valid_value"
		//     Expect(validator.ValidateCreate(ctx, obj)).To(BeNil())
		// })
		//
		// It("Should validate updates correctly", func() {
		//     By("simulating a valid update scenario")
		//     oldObj.SomeRequiredField = "updated_value"
		//     obj.SomeRequiredField = "updated_value"
		//     Expect(validator.ValidateUpdate(ctx, oldObj, obj)).To(BeNil())
		// })
	})

})
