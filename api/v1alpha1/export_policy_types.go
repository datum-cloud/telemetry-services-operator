// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for
// the fields to be serialized.

// ExportPolicySpec defines the desired state of ExportPolicy.
type ExportPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster Important: Run
	// "make" to regenerate code after modifying this file

	// Defines how the export policy should source telemetry data to publish to
	// the configured sinks. An export policy can define multiple telemetry
	// sources. The export policy will **not** de-duplicate telemetry data that
	// matches multiple sources.
	Sources []TelemetrySource `json:"sources"`

	// Configures how telemetry data should be sent to a third-party telemetry
	// platforms.
	Sink TelemetrySink `json:"sink"`
}

// ExportPolicyStatus defines the observed state of ExportPolicy.
type ExportPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Provides status information on the current status of the sink. This can be
	// used to determine whether a sink is configured correctly and is exporting
	// telemetry data.
	//
	// Known condition types are: "Ready"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ExportPolicy is the Schema for the export policy API.
type ExportPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Describes the expected state of the ExportPolicy's configuration. The
	// control plane will constantly evaluate the current state of exporters that
	// are deployed and ensure it matches the expected configuration. This field
	// is required when configuring an export policy.
	Spec ExportPolicySpec `json:"spec"`

	// Provides information on the current state of the export policy that was
	// observed by the control plane. This will be continuously updated as the
	// control plane monitors exporters.
	Status ExportPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExportPolicyList contains a list of ExportPolicy.
type ExportPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExportPolicy `json:"items"`
}

// A metric source configures the metric data that should be exported to the
// configured sinks. The options below are expected to be mutually exclusive.
type MetricSource struct {
	// The MetricSQL option allows to user to provide a metricsql query that can
	// be used to select and filter metric data that should be published by the
	// export policy.
	//
	// Here's an example of a metricsql query that will publish gateway metrics:
	//
	// ``` {service_name=“networking.datumapis.com”, resource_kind="Gateway"} ```
	//
	// See: https://docs.victoriametrics.com/metricsql/
	Metricsql string `json:"metricsql,omitempty"`
}

// Defines how the export policy should source telemetry data from resources on
// the platform.
type TelemetrySource struct {
	// A unique name given to the telemetry source within an export policy. Must
	// be a valid DNS label.
	Name string `json:"name"`

	// Configures how the telemetry source should retrieve metric data from the
	// Datum Cloud platform.
	Metrics *MetricSource `json:"metrics,omitempty"`
}

// Configures how telemetry data should be sent to a third-party platform. As of
// now there are no guarantees around delivery of telemetry data, especially if
// the sink's endpoint is unavailable.
type TelemetrySink struct {
	// Configures how telemetry data should be batched before sending to the sink.
	Batch Batch `json:"batch,omitempty"`

	// Configures the export policies' retry behavior when it fails to send
	// requests to the sink's endpoint. There's no guarantees that the export
	// policy will retry until success if the endpoint is not available or
	// configured incorrectly.
	Retry Retry `json:"retry,omitempty"`

	// Configures the export policy to publish telemetry using the HTTP version of
	// the OTLP protocol.
	//
	// See: https://opentelemetry.io/docs/specs/otel/protocol/
	OpenTelemetry *OpenTelemetrySink `json:"openTelemetry,omitempty"`
}

// References a secret in the same namespace as the entity defining the
// reference.
type LocalSecretReference struct {
	// The name of the secret
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// The key within the secret that contains the necessary value.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// Configures how Bearer token authentication should be used to authenticate
// with a sink's endpoint. This should be used when the endpoint requires an
// Authorization header in the following format:
//
// ``` Authorization: Bearer ... ```
type BearerTokenAuthentication struct {
	// Configures which secret is used to retrieve the bearer token to add to the
	// authorization header.
	SecretRef LocalSecretReference `json:"secretRef"`
}

// Configures how the sink will authenticate with the configured endpoint. These
// options are mutually exclusive.
type Authentication struct {
	// Configures the sink to use a Bearer token in the authorization header when
	// authenticating with the configured endpoint.
	BearerToken *BearerTokenAuthentication `json:"bearerToken,omitempty"`
}

// Configures how the sink should send data to a OTLP HTTP endpoint.
type OpenTelemetrySink struct {
	// Configures how the sink should authenticate with the HTTP endpoint.
	Authentication Authentication `json:"authentication,omitempty"`

	// Configure an HTTP endpoint to use for publishing telemetry data.
	HTTP *OpenTelemetryHTTP `json:"http,omitempty"`
}

// Configures the OpenTelemetry sink to use an HTTP endpoint to send telemetry
// data.
type OpenTelemetryHTTP struct {
	// The HTTP endpoint that should be used to publish telemetry data.
	Endpoint string `json:"endpoint"`
}

// Configures the batching behavior the sink will use to batch requests before
// publishing them to the endpoint.
type Batch struct {
	// Batch timeout before sending telemetry. Must be a duration (e.g. 5s).
	//
	// +kubebuilder:default="5s"
	Timeout string `json:"timeout"`
	// Maximum number of telemetry entries per batch.
	//
	// +kubebuilder:default=500
	MaxSize int `json:"maxSize"`
}

// Configures the retry behavior of the sink when it fails to send telemetry
// data to the configured endpoint.
type Retry struct {
	// Maximum number of attempts before telemetry data should be dropped.
	//
	// +kubebuilder:default=3
	MaxAttempts int `json:"maxAttempts"`
	// Backoff duration that should be used to backoff when retrying requests.
	// Should be a duration string, e.g. `10s`.
	//
	// +kubebuilder:default="5s"
	BackoffDuration string `json:"backoffDuration"`
}

func init() {
	SchemeBuilder.Register(&ExportPolicy{}, &ExportPolicyList{})
}
