// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
)

// ExportPolicyReconciler reconciles a ExportPolicy object
type ExportPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExportPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.ExportPolicy{}).
		Named("exportpolicy").
		Complete(r)
}
