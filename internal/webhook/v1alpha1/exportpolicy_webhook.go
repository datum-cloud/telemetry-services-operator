// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/VictoriaMetrics/metricsql"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"go.datum.net/telemetry-services-operator/api/v1alpha1"
	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var exportpolicylog = logf.Log.WithName("exportpolicy-resource")

// These labels can not be filtered in on a metricsql query.
var forbiddenLabels = []string{
	"project_name",
}

// SetupExportPolicyWebhookWithManager registers the webhook for ExportPolicy in the manager.
func SetupExportPolicyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&telemetryv1alpha1.ExportPolicy{}).
		WithValidator(&ExportPolicyCustomValidator{}).
		WithDefaulter(&ExportPolicyCustomDefaulter{
			DefaultRetryMaxAttempts:     3,
			DefaultRetryBackoffDuration: "2s",
			DefaultBatchTimeout:         "5s",
			DefaultBatchMaxSize:         500,
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-telemetry-datumapis-com-v1alpha1-exportpolicy,mutating=true,failurePolicy=fail,sideEffects=None,groups=telemetry.datumapis.com,resources=exportpolicies,verbs=create;update,versions=v1alpha1,name=mexportpolicy-v1alpha1.kb.io,admissionReviewVersions=v1

// ExportPolicyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ExportPolicy when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ExportPolicyCustomDefaulter struct {
	DefaultRetryMaxAttempts     int
	DefaultRetryBackoffDuration string
	DefaultBatchTimeout         string
	DefaultBatchMaxSize         int
}

var _ webhook.CustomDefaulter = &ExportPolicyCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ExportPolicy.
func (d *ExportPolicyCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	exportpolicy, ok := obj.(*telemetryv1alpha1.ExportPolicy)

	if !ok {
		return fmt.Errorf("expected an ExportPolicy object but got %T", obj)
	}
	exportpolicylog.Info("Defaulting for ExportPolicy", "name", exportpolicy.GetName())

	for i := range exportpolicy.Spec.Sinks {
		if exportpolicy.Spec.Sinks[i].Retry.MaxAttempts == 0 {
			exportpolicy.Spec.Sinks[i].Retry.MaxAttempts = d.DefaultRetryMaxAttempts
		}
		if exportpolicy.Spec.Sinks[i].Retry.BackoffDuration == "" {
			exportpolicy.Spec.Sinks[i].Retry.BackoffDuration = d.DefaultRetryBackoffDuration
		}
		if exportpolicy.Spec.Sinks[i].Batch.MaxSize == 0 {
			exportpolicy.Spec.Sinks[i].Batch.MaxSize = d.DefaultBatchMaxSize
		}
		if exportpolicy.Spec.Sinks[i].Batch.Timeout == "" {
			exportpolicy.Spec.Sinks[i].Batch.Timeout = d.DefaultBatchTimeout
		}
	}

	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-telemetry-datumapis-com-v1alpha1-exportpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=telemetry.datumapis.com,resources=exportpolicies,verbs=create;update,versions=v1alpha1,name=vexportpolicy-v1alpha1.kb.io,admissionReviewVersions=v1

// ExportPolicyCustomValidator struct is responsible for validating the ExportPolicy resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ExportPolicyCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ExportPolicyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ExportPolicy.
func (v *ExportPolicyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	exportpolicy, ok := obj.(*telemetryv1alpha1.ExportPolicy)
	if !ok {
		return nil, fmt.Errorf("expected a ExportPolicy object but got %T", obj)
	}
	exportpolicylog.Info("Validation for ExportPolicy upon creation", "name", exportpolicy.GetName())

	if errs := validateExportPolicy(exportpolicy); len(errs) > 0 {
		return nil, errors.NewInvalid(obj.GetObjectKind().GroupVersionKind().GroupKind(), exportpolicy.Name, errs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ExportPolicy.
func (v *ExportPolicyCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	exportpolicy, ok := newObj.(*telemetryv1alpha1.ExportPolicy)
	if !ok {
		return nil, fmt.Errorf("expected a ExportPolicy object for the newObj but got %T", newObj)
	}
	exportpolicylog.Info("Validation for ExportPolicy upon update", "name", exportpolicy.GetName())

	if errs := validateExportPolicy(exportpolicy); len(errs) > 0 {
		return nil, errors.NewInvalid(newObj.GetObjectKind().GroupVersionKind().GroupKind(), exportpolicy.Name, errs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ExportPolicy.
func (v *ExportPolicyCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	exportpolicy, ok := obj.(*telemetryv1alpha1.ExportPolicy)
	if !ok {
		return nil, fmt.Errorf("expected a ExportPolicy object but got %T", obj)
	}
	exportpolicylog.Info("Validation for ExportPolicy upon deletion", "name", exportpolicy.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

func validateExportPolicy(policy *v1alpha1.ExportPolicy) field.ErrorList {
	return validateExportPolicySpec(field.NewPath("spec"), policy.Spec)
}

func validateExportPolicySpec(fieldPath *field.Path, spec v1alpha1.ExportPolicySpec) field.ErrorList {
	var errs field.ErrorList
	if len(spec.Sources) == 0 {
		errs = append(errs, field.Required(fieldPath.Child("sources"), "At least one telemetry source is required"))
	} else {
		sourceNames := map[string]struct{}{}

		for index, source := range spec.Sources {
			sourcePath := fieldPath.Child("sources").Index(index)
			if source.Name == "" {
				errs = append(errs, field.Required(sourcePath.Key("name"), "A unique name for the resource is required"))
			} else if _, set := sourceNames[source.Name]; set {
				errs = append(errs, field.Duplicate(sourcePath.Child("name"), source.Name))
			} else {
				sourceNames[source.Name] = struct{}{}
			}

			if source.Metrics == nil {
				errs = append(errs, field.Required(sourcePath.Child("metrics"), "A source must provide a metrics source. Additional source types will be supported in the future."))
			} else {
				errs = append(errs, validateMetricSource(sourcePath.Child("metrics"), *source.Metrics)...)
			}
		}
	}

	for _, sink := range spec.Sinks {
		errs = append(errs, validateTelemetrySink(fieldPath.Child("sink"), sink)...)
	}

	return errs
}

func validateMetricSource(path *field.Path, metrics v1alpha1.MetricSource) field.ErrorList {
	var errs field.ErrorList
	if metrics.Metricsql == "" {
		errs = append(errs, field.Required(path.Child("metricsql"), "A metricsql query is required. Additional metric options will be supported in the future."))
	} else {
		expr, err := metricsql.Parse(metrics.Metricsql)
		if err != nil {
			errs = append(errs, field.Invalid(path.Child("metricsql"), metrics.Metricsql, fmt.Sprintf("Invalid metricsql query provided: %s", err)))
		} else if metricExpr, ok := expr.(*metricsql.MetricExpr); !ok {
			errs = append(errs, field.Invalid(path.Child("metricsql"), metrics.Metricsql, `Only metrics queries in the format '{label="value"}' are supported`))
		} else {
			for _, labelFilters := range metricExpr.LabelFilterss {
				for _, labelFilter := range labelFilters {
					if slices.Contains(forbiddenLabels, labelFilter.Label) {
						errs = append(errs, field.Invalid(path.Child("metricsql"), metrics.Metricsql, fmt.Sprintf("The metricsql cannot contain the label '%s'", labelFilter.Label)))
					}
				}
			}
		}
	}

	return errs
}

func validateTelemetrySink(path *field.Path, sink v1alpha1.TelemetrySink) field.ErrorList {
	var errs field.ErrorList
	if sink.PrometheusRemoteWrite == nil {
		errs = append(errs, field.Required(path.Child("prometheusRemoteWrite"), ""))
	} else {
		errs = append(errs, validatePrometheusRemoteWrite(path.Child("prometheusRemoteWrite"), *sink.PrometheusRemoteWrite)...)
	}

	batchPath := path.Child("batch")
	if sink.Batch.MaxSize > 1000 || sink.Batch.MaxSize < 1 {
		errs = append(errs, field.Invalid(batchPath.Child("maxSize"), sink.Batch.MaxSize, "must be between 1 and 1000"))
	}
	if timeout, err := time.ParseDuration(sink.Batch.Timeout); err != nil {
		errs = append(errs, field.Invalid(batchPath.Child("timeout"), sink.Batch.Timeout, "must be a valid duration string (e.g. '5s')"))
	} else if timeout > 10*time.Second || timeout < time.Second {
		errs = append(errs, field.Invalid(batchPath.Child("timeout"), sink.Batch.Timeout, "must be between 1s and 10s"))
	}

	retryPath := path.Child("retry")
	if sink.Retry.MaxAttempts > 5 || sink.Retry.MaxAttempts < 1 {
		errs = append(errs, field.Invalid(retryPath.Child("maxAttempts"), sink.Retry.MaxAttempts, "must be between 1 and 5"))
	}
	if backoffDuration, err := time.ParseDuration(sink.Retry.BackoffDuration); err != nil {
		errs = append(errs, field.Invalid(retryPath.Child("backoffDuration"), sink.Retry.BackoffDuration, "must be a valid duration string (e.g. '5s')"))
	} else if backoffDuration > 10*time.Second || backoffDuration < time.Second {
		errs = append(errs, field.Invalid(retryPath.Child("backoffDuration"), sink.Retry.BackoffDuration, "must be between 1s and 10s"))
	}

	return errs
}

func validatePrometheusRemoteWrite(path *field.Path, otel v1alpha1.PrometheusRemoteWriteSink) field.ErrorList {
	var errs field.ErrorList
	if otel.Endpoint == "" {
		errs = append(errs, field.Required(path.Child("http"), "A valid endpoint URL is required"))
	} else if _, err := url.Parse(otel.Endpoint); err != nil {
		errs = append(errs, field.Invalid(path.Child("http"), otel.Endpoint, fmt.Sprintf("Failed to parse URL: %s", err)))
	}
	return errs
}
