// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.datum.net/telemetry-services-operator/api/v1alpha1"
)

// ExportPolicyReconciler reconciles a ExportPolicy object
type ExportPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// The metrics service that can be used to query metrics from the telemetry
	// system.
	MetricsService MetricsService

	// The vector config label key that will be added to the vector config secret.
	VectorConfigLabelKey   string
	VectorConfigLabelValue string
}

// MetricsService is a struct that contains the information needed to configure
// a metrics service.
type MetricsService struct {
	// The endpoint of the metrics service that can be used to query metrics
	// from the telemetry system.
	Endpoint string
	// The username for the metrics service.
	Username string
	// The password for the metrics service.
	Password string
}

// +kubebuilder:rbac:groups=telemetry.datumapis.com,resources=exportpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.datumapis.com,resources=exportpolicies/status,verbs=get;update;patch

// Reconcile an Export Policy and ensure the necessary resources exist to export
// the telemetry sources that are configured. This will create a vector config
// secret that can be used to configure the vector exporter to export the
// telemetry sources to the configured sinks. The vector config secret will be
// named `export-policy-vector-config-<export-policy-uid>.json`. The vector
// config secret will be created in the same namespace as the export policy.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *ExportPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Retrieve the export policy from the cluster and confirm if it exists.
	exportPolicy := &v1alpha1.ExportPolicy{}
	if err := r.Client.Get(ctx, req.NamespacedName, exportPolicy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	logger.Info("reconciling export policy")

	// Configure the export policy and update the status information for the sink
	// based on whether the sink was correctly configured.
	// Create a vector configuration for each source and sink combination
	vectorConfig := map[string]any{
		"sources": make(map[string]any),
		"sinks":   make(map[string]any),
	}

	// Configure the sources that will be used to export the metrics from the
	// telemetry sources to the configured sinks.
	sources := vectorConfig["sources"].(map[string]any)
	for _, source := range exportPolicy.Spec.Sources {
		sourceID := fmt.Sprintf("%s:%s", exportPolicy.UID, source.Name)

		if source.Metrics == nil {
			// TODO: Add a status condition to the export policy to indicate that
			// the telemetry source is not supported.
			return ctrl.Result{}, fmt.Errorf("unsupported telemetry source type")
		}

		sources[sourceID] = map[string]any{
			"type":      "prometheus_scrape",
			"endpoints": []string{r.MetricsService.Endpoint},
			"auth": map[string]any{
				"strategy": "basic",
				"user":     r.MetricsService.Username,
				"password": r.MetricsService.Password,
			},
			"query": map[string]any{
				"match[]": []string{source.Metrics.MetricsQL},
			},
		}
	}

	// Configure sinks
	sinks := vectorConfig["sinks"].(map[string]any)
	sinkStatuses := []v1alpha1.SinkStatus{}
	var anyStatusChanged bool

	for _, sink := range exportPolicy.Spec.Sinks {
		sinkID := fmt.Sprintf("%s:%s", exportPolicy.UID, sink.Name)
		sinkConfig, sinkStatus, statusChanged := configureSink(ctx, r.Client, sink, exportPolicy)

		anyStatusChanged = anyStatusChanged || statusChanged
		sinkStatuses = append(sinkStatuses, *sinkStatus)
		sinks[sinkID] = sinkConfig
	}

	statusChanged := updateExportPolicyConditions(exportPolicy, sinkStatuses)

	// Update export policy status if needed
	if anyStatusChanged || statusChanged {
		exportPolicy.Status.Sinks = sinkStatuses

		if err := r.Client.Status().Update(ctx, exportPolicy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update export policy status: %w", err)
		}
	}

	// Create or update the vector config secret
	vectorConfigJSON, err := json.MarshalIndent(vectorConfig, "", "  ")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal vector config: %w", err)
	}

	configSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("export-policy-vector-config-%s", exportPolicy.GetUID()),
			Namespace: exportPolicy.GetNamespace(),
			Labels: map[string]string{
				r.VectorConfigLabelKey: r.VectorConfigLabelValue,
			},
		},
		Data: map[string][]byte{
			fmt.Sprintf("%s.json", exportPolicy.UID): vectorConfigJSON,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, configSecret, func() error {
		// Set the owner reference for the vector config secret so it is deleted
		// when the export policy is deleted.
		if err := controllerutil.SetControllerReference(exportPolicy, configSecret, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		configSecret.Data = map[string][]byte{
			fmt.Sprintf("%s.json", exportPolicy.UID): vectorConfigJSON,
		}

		return nil
	})

	return ctrl.Result{}, err
}

// configureSink sets up a sink configuration and returns the sink
// configuration, status, and whether the status changed
func configureSink(ctx context.Context, client client.Client, sink v1alpha1.TelemetrySink, exportPolicy *v1alpha1.ExportPolicy) (map[string]any, *v1alpha1.SinkStatus, bool) {
	logger := log.FromContext(ctx)

	sinkStatus := getSinkStatus(exportPolicy, sink.Name)
	var statusChanged bool

	if sink.Target.PrometheusRemoteWrite != nil {
		// Get all of the sources that are configured for the sink and add them
		// to the inputs for the prometheus remote write sink.
		inputs := []string{}
		for _, source := range sink.Sources {
			inputs = append(inputs, fmt.Sprintf("%s:%s", exportPolicy.UID, source))
		}

		// Configure the prometheus remote write sink
		sinkConfig := map[string]any{
			"type":     "prometheus_remote_write",
			"endpoint": sink.Target.PrometheusRemoteWrite.Endpoint,
			"inputs":   inputs,
		}

		if sink.Target.PrometheusRemoteWrite.Authentication != nil {
			// Validate the authentication secret exists and is the correct type
			secretRef := sink.Target.PrometheusRemoteWrite.Authentication.BasicAuth.SecretRef
			secret := &corev1.Secret{}
			err := client.Get(ctx, types.NamespacedName{
				Name:      secretRef.Name,
				Namespace: exportPolicy.Namespace,
			}, secret)

			if err != nil || secret.Type != "kubernetes.io/basic-auth" {
				condition := metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: exportPolicy.Generation,
				}

				if errors.IsNotFound(err) {
					condition.Reason = "SecretNotFound"
					condition.Message = fmt.Sprintf("The configured secret `%s` was not found", secretRef.Name)
				} else if err != nil {
					condition.Reason = "SecretError"
					condition.Message = fmt.Sprintf("Failed to check if the secret '%s' exists", secretRef.Name)
					logger.Error(err, "failed to check if secret exists")
				} else if secret.Type != "kubernetes.io/basic-auth" {
					condition.Reason = "InvalidSecretType"
					condition.Message = fmt.Sprintf("Secret `%s` must be of type `kubernetes.io/basic-auth`", secretRef.Name)
				}

				if apimeta.SetStatusCondition(&sinkStatus.Conditions, condition) {
					statusChanged = true
				}
			}

			sinkConfig["auth"] = map[string]any{
				"strategy": "basic",
				"user":     string(secret.Data["username"]),
				"password": string(secret.Data["password"]),
			}
		}

		// Mark sink as ready
		if apimeta.SetStatusCondition(&sinkStatus.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Configured",
			ObservedGeneration: exportPolicy.Generation,
			Message:            "Sink configured successfully",
		}) {
			statusChanged = true
		}

		return sinkConfig, sinkStatus, statusChanged
	}

	// Handle unsupported sink type
	if apimeta.SetStatusCondition(&sinkStatus.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "UnsupportedSinkType",
		ObservedGeneration: exportPolicy.Generation,
		Message:            "Sink type not supported",
	}) {
		statusChanged = true
	}

	return nil, sinkStatus, statusChanged
}

// getSinkStatus retrieves the existing sink status from the export policy if it exists,
// otherwise returns a new sink status with the given name
func getSinkStatus(exportPolicy *v1alpha1.ExportPolicy, sinkName string) *v1alpha1.SinkStatus {
	for _, existingStatus := range exportPolicy.Status.Sinks {
		if existingStatus.Name == sinkName {
			return existingStatus.DeepCopy()
		}
	}
	return &v1alpha1.SinkStatus{
		Name: sinkName,
	}
}

// updateExportPolicyConditions updates the overall status conditions of the export policy
// based on the status of its sinks. Returns true if conditions were changed.
func updateExportPolicyConditions(exportPolicy *v1alpha1.ExportPolicy, sinkStatuses []v1alpha1.SinkStatus) bool {
	var readyCount, failedCount int
	for _, sinkStatus := range sinkStatuses {
		for _, condition := range sinkStatus.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == metav1.ConditionTrue {
					readyCount++
				} else {
					failedCount++
				}
			}
		}
	}

	var condition metav1.Condition
	if readyCount == len(sinkStatuses) {
		condition = metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "SinksReady",
			Message:            "All sinks are configured.",
			ObservedGeneration: exportPolicy.Generation,
		}
	} else {
		condition = metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "SinksNotReady",
			Message:            fmt.Sprintf("%d/%d sinks are ready. Check the status of the sinks for more details.", readyCount, len(sinkStatuses)),
			ObservedGeneration: exportPolicy.Generation,
		}
	}

	return apimeta.SetStatusCondition(&exportPolicy.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExportPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ExportPolicy{}).
		Named("exportpolicy").
		Complete(r)
}
