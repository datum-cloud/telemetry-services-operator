// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var exporterlog = logf.Log.WithName("exporter-resource")

// SetupExportPolicyWebhookWithManager registers the webhook for ExportPolicy in the manager.
func SetupExportPolicyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&telemetryv1alpha1.ExportPolicy{}).
		WithDefaulter(&ExportPolicyCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-telemetry-datumapis-com-v1alpha1-exporter,mutating=true,failurePolicy=fail,sideEffects=None,groups=telemetry.datumapis.com,resources=exporters,verbs=create;update,versions=v1alpha1,name=mexporter-v1alpha1.kb.io,admissionReviewVersions=v1

// ExportPolicyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ExportPolicy when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ExportPolicyCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &ExportPolicyCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ExportPolicy.
func (d *ExportPolicyCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	exporter, ok := obj.(*telemetryv1alpha1.ExportPolicy)

	if !ok {
		return fmt.Errorf("expected an ExportPolicy object but got %T", obj)
	}
	exporterlog.Info("Defaulting for ExportPolicy", "name", exporter.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}
