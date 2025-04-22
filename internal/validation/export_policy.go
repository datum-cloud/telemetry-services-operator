package validation

import (
	"fmt"
	"net/url"
	"slices"

	"github.com/VictoriaMetrics/metricsql"
	"k8s.io/apimachinery/pkg/util/validation/field"

	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
)

// These labels can not be filtered in on a metricsql query.
var forbiddenLabels = []string{
	"project_name",
}

func ValidateExportPolicy(policy *telemetryv1alpha1.ExportPolicy) field.ErrorList {
	return validateExportPolicySpec(field.NewPath("spec"), policy.Spec)
}

func validateExportPolicySpec(fieldPath *field.Path, spec telemetryv1alpha1.ExportPolicySpec) field.ErrorList {
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

	sinkNames := map[string]struct{}{}
	for index, sink := range spec.Sinks {
		// Validate that the sink name is unique
		sinkPath := fieldPath.Child("sinks").Index(index)
		if _, set := sinkNames[sink.Name]; set {
			errs = append(errs, field.Duplicate(sinkPath.Child("name"), sink.Name))
		} else {
			sinkNames[sink.Name] = struct{}{}
		}

		errs = append(errs, validateTelemetrySink(sinkPath, sink)...)
	}

	return errs
}

func validateMetricSource(path *field.Path, metrics telemetryv1alpha1.MetricSource) field.ErrorList {
	var errs field.ErrorList
	if metrics.MetricsQL == "" {
		errs = append(errs, field.Required(path.Child("metricsql"), "A metricsql query is required. Additional metric options will be supported in the future."))
	} else {
		expr, err := metricsql.Parse(metrics.MetricsQL)
		if err != nil {
			errs = append(errs, field.Invalid(path.Child("metricsql"), metrics.MetricsQL, fmt.Sprintf("Invalid metricsql query provided: %s", err)))
		} else if metricExpr, ok := expr.(*metricsql.MetricExpr); !ok {
			errs = append(errs, field.Invalid(path.Child("metricsql"), metrics.MetricsQL, `Only metrics queries in the format '{label="value"}' are supported`))
		} else {
			for _, labelFilters := range metricExpr.LabelFilterss {
				for _, labelFilter := range labelFilters {
					if slices.Contains(forbiddenLabels, labelFilter.Label) {
						errs = append(errs, field.Invalid(path.Child("metricsql"), metrics.MetricsQL, fmt.Sprintf("The metricsql cannot contain the label '%s'", labelFilter.Label)))
					}
				}
			}
		}
	}

	return errs
}

func validateTelemetrySink(path *field.Path, sink telemetryv1alpha1.TelemetrySink) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, validateTelemetrySinkTarget(path.Child("target"), *sink.Target)...)
	return errs
}

func validateTelemetrySinkTarget(path *field.Path, sink telemetryv1alpha1.SinkTarget) field.ErrorList {
	var errs field.ErrorList
	if sink.PrometheusRemoteWrite == nil {
		errs = append(errs, field.Required(path.Child("prometheusRemoteWrite"), ""))
	} else {
		errs = append(errs, validatePrometheusRemoteWrite(path.Child("prometheusRemoteWrite"), *sink.PrometheusRemoteWrite)...)
	}

	return errs
}

func validatePrometheusRemoteWrite(path *field.Path, otel telemetryv1alpha1.PrometheusRemoteWriteSink) field.ErrorList {
	var errs field.ErrorList
	if otel.Endpoint == "" {
		errs = append(errs, field.Required(path.Child("http"), "A valid endpoint URL is required"))
	} else if _, err := url.Parse(otel.Endpoint); err != nil {
		errs = append(errs, field.Invalid(path.Child("http"), otel.Endpoint, fmt.Sprintf("Failed to parse URL: %s", err)))
	}
	return errs
}
