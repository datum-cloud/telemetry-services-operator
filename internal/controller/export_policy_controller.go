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
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;update;delete

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

	// Validate that the export policy configuration is valid and update the
	// status of the export policy to reflect the status of the sinks.
	if statusChanged, err := r.validateExportPolicy(ctx, exportPolicy); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to validate export policy: %w", err)
	} else if statusChanged {
		logger.Info("export policy status changed, updating status")
		if err := r.Client.Status().Update(ctx, exportPolicy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update export policy status: %w", err)
		}
	}

	// Configure the export policy and update the status information for the sink
	// based on whether the sink was correctly configured.
	vectorConfig, err := r.createVectorConfiguration(ctx, exportPolicy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create vector configuration: %w", err)
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

// validateExportPolicy validates the export policy configuration and updates the
// status of the export policy to reflect the status of the sinks.
func (r *ExportPolicyReconciler) validateExportPolicy(ctx context.Context, exportPolicy *v1alpha1.ExportPolicy) (bool, error) {
	statusChanged := false
	sinkStatuses := []v1alpha1.SinkStatus{}
	// Validate each of the sinks in the export policy have a valid configuration
	// and the secrets exist if necessary.
	for _, sink := range exportPolicy.Spec.Sinks {
		status := getSinkStatus(exportPolicy, sink.Name)

		if sink.Target.PrometheusRemoteWrite != nil {
			// Assume the sink is accepted and expect to be set to false if any
			// validation fails.
			accepted := true

			// Validate that any authentication for the sink is valid
			if sink.Target.PrometheusRemoteWrite.Authentication != nil {
				if sink.Target.PrometheusRemoteWrite.Authentication.BasicAuth != nil {
					_, err := retrieveBasicAuthSecret(ctx, r.Client, sink.Target.PrometheusRemoteWrite.Authentication.BasicAuth.SecretRef, exportPolicy)
					if err != nil {
						accepted = false
						updated := apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
							Type:    "Accepted",
							Status:  metav1.ConditionFalse,
							Reason:  "InvalidAuthentication",
							Message: err.Error(),
						})

						if updated {
							statusChanged = true
						}
					}
				}
			}

			if accepted {
				updated := apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
					Type:   "Accepted",
					Status: metav1.ConditionTrue,
					Reason: "SinkConfigured",
				})

				if updated {
					statusChanged = true
				}
			}
		}

		sinkStatuses = append(sinkStatuses, *status)
	}

	exportPolicy.Status.Sinks = sinkStatuses

	// Update the overall status conditions of the export policy based on the
	// status of its sinks.
	statusChanged = updateExportPolicyConditions(exportPolicy, sinkStatuses) || statusChanged

	return statusChanged, nil
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
	var acceptedCount int
	for _, sinkStatus := range sinkStatuses {
		for _, condition := range sinkStatus.Conditions {
			if condition.Type == "Accepted" {
				if condition.Status == metav1.ConditionTrue {
					acceptedCount++
				}
			}
		}
	}

	var condition metav1.Condition
	if acceptedCount == len(sinkStatuses) {
		condition = metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "SinksAccepted",
			Message:            "All sinks are configured.",
			ObservedGeneration: exportPolicy.Generation,
		}
	} else {
		condition = metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "SinksNotAccepted",
			Message:            fmt.Sprintf("%d/%d sinks are accepted. Check the status of the sinks for more details.", acceptedCount, len(sinkStatuses)),
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
