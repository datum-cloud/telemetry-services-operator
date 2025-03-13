// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.datum.net/telemetry-services-operator/api/v1alpha1"
	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
)

const exportPolicyFinalizer = "telemetry.datumapis.com/export-policy-controller"

// ExportPolicyReconciler reconciles a ExportPolicy object
type ExportPolicyReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	finalizers finalizer.Finalizers
}

// +kubebuilder:rbac:groups=telemetry.datumapis.com,resources=exportpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.datumapis.com,resources=exportpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=telemetry.datumapis.com,resources=exportpolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ExportPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *ExportPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"request_name", req.Name,
		"request_namespace", req.Namespace,
	)

	var exportPolicy v1alpha1.ExportPolicy
	if err := r.Client.Get(ctx, req.NamespacedName, &exportPolicy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	logger.Info("reconciling export policy")

	finalizationResult, err := r.finalizers.Finalize(ctx, &exportPolicy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, &exportPolicy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
		return ctrl.Result{}, nil
	}

	apimeta.SetStatusCondition(&exportPolicy.Status.Conditions, v1.Condition{
		Type:   "Ready",
		Status: v1.ConditionTrue,
	})

	return ctrl.Result{}, nil
}

func (r *ExportPolicyReconciler) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	// TODO: Clean up vector deployment
	return finalizer.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExportPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(exportPolicyFinalizer, r); err != nil {
		return fmt.Errorf("failed to register finalizer")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.ExportPolicy{}).
		Named("exportpolicy").
		Complete(r)
}
