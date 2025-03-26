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
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.datum.net/telemetry-services-operator/api/v1alpha1"
)

const exportPolicyFinalizer = "telemetry.datumapis.com/export-policy-controller"

// ExportPolicyReconciler reconciles a ExportPolicy object
type ExportPolicyReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	finalizers finalizer.Finalizers

	// The metrics service that can be used to query metrics from the telemetry
	// system.
	MetricsService MetricsService
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
// +kubebuilder:rbac:groups=telemetry.datumapis.com,resources=exportpolicies/finalizers,verbs=update

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
	logger := log.FromContext(ctx).WithValues(
		"request_name", req.Name,
		"request_namespace", req.Namespace,
	)

	// Retrieve the export policy from the cluster and confirm if it exists.
	exportPolicy := &v1alpha1.ExportPolicy{}
	if err := r.Client.Get(ctx, req.NamespacedName, exportPolicy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	logger.Info("reconciling export policy")

	// Ensure the export policy is finalized so we can clean up resources when the
	// policy is deleted.
	finalizationResult, err := r.finalizers.Finalize(ctx, exportPolicy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, exportPolicy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
		return ctrl.Result{}, nil
	}

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
				"match[]": []string{source.Metrics.Metricsql},
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
				"export-policy-vector-config": "true",
			},
		},
		Data: map[string][]byte{
			fmt.Sprintf("%s.json", exportPolicy.UID): vectorConfigJSON,
		},
	}

	// Create the secret if it doesn't exist, otherwise update it.
	if err := r.Client.Create(ctx, configSecret); err != nil {
		if !errors.IsAlreadyExists(err) {
			return ctrl.Result{}, fmt.Errorf("failed to create vector config secret: %w", err)
		}
		// Update existing secret
		if err := r.Client.Update(ctx, configSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update vector config secret: %w", err)
		}
	}

	return ctrl.Result{}, err
}

// Finalize will ensure all resources related to the export policy are deleted
// from the cluster before the resource can be deleted.
func (r *ExportPolicyReconciler) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("export-policy-vector-config-%s", obj.GetUID()),
		Namespace: obj.GetNamespace(),
	}, secret); errors.IsNotFound(err) {
		return finalizer.Result{}, nil
	} else if err != nil {
		return finalizer.Result{}, err
	}

	// Delete the vector config secret
	if err := r.Client.Delete(ctx, secret); err != nil {
		return finalizer.Result{}, fmt.Errorf("failed to delete vector config secret: %w", err)
	}

	return finalizer.Result{}, nil
}

// configureSink sets up a sink configuration and returns the sink
// configuration, status, and whether the status changed
func configureSink(ctx context.Context, client client.Client, sink v1alpha1.TelemetrySink, exportPolicy *v1alpha1.ExportPolicy) (map[string]any, *v1alpha1.SinkStatus, bool) {
	logger := log.FromContext(ctx)

	sinkStatus := getSinkStatus(exportPolicy, sink.Name)
	var statusChanged bool

	if sink.PrometheusRemoteWrite != nil {
		// Get all of the sources that are configured for the sink and add them
		// to the inputs for the prometheus remote write sink.
		inputs := []string{}
		for _, source := range sink.Sources {
			inputs = append(inputs, fmt.Sprintf("%s:%s", exportPolicy.UID, source))
		}

		// Configure the prometheus remote write sink
		sinkConfig := map[string]any{
			"type":     "prometheus_remote_write",
			"endpoint": sink.PrometheusRemoteWrite.Endpoint,
			"inputs":   inputs,
		}

		if sink.PrometheusRemoteWrite.Authentication != nil {
			// Validate the authentication secret exists and is the correct type
			secretRef := sink.PrometheusRemoteWrite.Authentication.BasicAuth.SecretRef
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
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(exportPolicyFinalizer, r); err != nil {
		return fmt.Errorf("failed to register finalizer")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ExportPolicy{}).
		Named("exportpolicy").
		Complete(r)
}
