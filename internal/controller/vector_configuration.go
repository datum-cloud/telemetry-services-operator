package controller

import (
	"context"
	"fmt"
	"maps"

	"github.com/VictoriaMetrics/metricsql"
	"go.datum.net/telemetry-services-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// createVectorConfiguration creates a vector configuration for the export policy
// and returns the vector configuration as a map. The vector configuration is
// used to configure the vector exporter to export the telemetry sources to
// the configured sinks.
//
// This will only configure sources and sinks that are considered valid. Any
// invalid sources or sinks will be skipped. It's expected that the export
// policy configuration is validated before this function is called and that the
// status of the export policy will be updated to highlight any issues with the
// export policy configuration.
func (r *ExportPolicyReconciler) createVectorConfiguration(ctx context.Context, projectName string, client client.Client, exportPolicy *v1alpha1.ExportPolicy) map[string]any {
	// Create a vector configuration for each source and sink combination
	vectorConfig := map[string]any{
		"sources": make(map[string]any),
		"sinks":   make(map[string]any),
	}

	// Configure the sources that will be used to export the metrics from the
	// telemetry sources to the configured sinks.
	sources := vectorConfig["sources"].(map[string]any)
	for _, source := range exportPolicy.Spec.Sources {
		if source.Metrics == nil {
			continue
		}

		query, err := metricsql.Parse(source.Metrics.MetricsQL)
		if err != nil {
			log.FromContext(ctx, "source", source.Name).Error(err, "unable to parse metricsql query")
			continue
		}

		metricExpr, ok := query.(*metricsql.MetricExpr)
		if !ok {
			log.FromContext(ctx, "source", source.Name).Error(fmt.Errorf("failed to convert metricsql query to MetricExpress"), "")
		}

		// Default to the project name as the label filter so that all metrics for
		// the project are exported.
		if len(metricExpr.LabelFilterss) == 0 {
			metricExpr.LabelFilterss = [][]metricsql.LabelFilter{
				{
					{
						Label: "resourcemanager_datumapis_com_project_name",
						Value: projectName,
					},
				},
			}
		} else {
			for i := range metricExpr.LabelFilterss {
				metricExpr.LabelFilterss[i] = append(metricExpr.LabelFilterss[i], metricsql.LabelFilter{
					Label: "resourcemanager_datumapis_com_project_name",
					Value: projectName,
				})
			}
		}

		marshalledQuery := []byte{}
		marshalledQuery = metricExpr.AppendString(marshalledQuery)

		sources[getVectorComponentID(exportPolicy, projectName, source.Name)] = map[string]any{
			"type":      "prometheus_scrape",
			"endpoints": []string{r.MetricsService.Endpoint},
			"auth": map[string]any{
				"strategy": "basic",
				"user":     r.MetricsService.Username,
				"password": r.MetricsService.Password,
			},
			"query": map[string]any{
				"match[]": []string{string(marshalledQuery)},
			},
		}
	}

	// Configure sinks
	sinks := vectorConfig["sinks"].(map[string]any)

	for _, sink := range exportPolicy.Spec.Sinks {
		sinkConfig, err := getSinkVectorConfig(ctx, client, projectName, sink, exportPolicy)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to get vector configuration for sink", "sink", sink.Name)
			continue
		}

		sinks[getVectorComponentID(exportPolicy, projectName, sink.Name)] = sinkConfig
	}

	return vectorConfig
}

// getVectorComponentID will return the fully qualified ID of a source for an export
// policy. The ID will encode metadata from the export policy into the ID so
// it's available in the internal telemetry of the vector instance.
func getVectorComponentID(exportPolicy *v1alpha1.ExportPolicy, projectName, componentName string) string {
	return fmt.Sprintf("export-policy:%s:%s:%s:%s:%s", projectName, exportPolicy.Namespace, exportPolicy.Name, exportPolicy.UID, componentName)
}

// getSinkVectorConfig creates a vector configuration for the given sink.
func getSinkVectorConfig(ctx context.Context, client client.Client, projectName string, sink v1alpha1.TelemetrySink, exportPolicy *v1alpha1.ExportPolicy) (map[string]any, error) {
	config := map[string]any{}

	// Get all of the sources that are configured for the sink and add them
	// to the inputs for the prometheus remote write sink.
	inputs := []string{}
	for _, source := range sink.Sources {
		inputs = append(inputs, getVectorComponentID(exportPolicy, projectName, source))
	}
	config["inputs"] = inputs

	// Create the vector configuration for the sink
	if sink.Target.PrometheusRemoteWrite != nil {
		prometheusRemoteWriteConfig, err := getPrometheusRemoteWriteSinkVectorConfig(ctx, client, *sink.Target.PrometheusRemoteWrite, exportPolicy)
		if err != nil {
			return nil, err
		}

		// Merge the prometheus remote write config with the config
		maps.Copy(config, prometheusRemoteWriteConfig)
	} else {
		return nil, fmt.Errorf("sink %s is not a valid sink", sink.Name)
	}

	return config, nil
}

// getPrometheusRemoteWriteSinkVectorConfig creates a vector configuration for
// the prometheus remote write sink.
func getPrometheusRemoteWriteSinkVectorConfig(ctx context.Context, client client.Client, sink v1alpha1.PrometheusRemoteWriteSink, exportPolicy *v1alpha1.ExportPolicy) (map[string]any, error) {
	// Configure the prometheus remote write sink
	sinkConfig := map[string]any{
		"type":     "prometheus_remote_write",
		"endpoint": sink.Endpoint,
	}

	if sink.Authentication != nil {
		secret, err := retrieveBasicAuthSecret(ctx, client, sink.Authentication.BasicAuth.SecretRef, exportPolicy)
		if err != nil {
			return nil, err
		}

		sinkConfig["auth"] = map[string]any{
			"strategy": "basic",
			"user":     string(secret.Data["username"]),
			"password": string(secret.Data["password"]),
		}
	}

	return sinkConfig, nil
}

// retrieveBasicAuthSecret retrieves the basic auth secret for the prometheus.
// This will return an error if the secret does not exist, is not of the
// correct type, or if the secret data does not contain the expected keys.
func retrieveBasicAuthSecret(ctx context.Context, client client.Client, secretRef v1alpha1.LocalSecretReference, exportPolicy *v1alpha1.ExportPolicy) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      secretRef.Name,
		Namespace: exportPolicy.Namespace,
	}, secret)

	if errors.IsNotFound(err) {
		return nil, fmt.Errorf("secret '%s' not found", secretRef.Name)
	} else if err != nil {
		log.FromContext(ctx).Error(err, "failed to get secret", "secret", secretRef.Name)
		return nil, fmt.Errorf("internal error when retrieving secret")
	} else if secret.Type != "kubernetes.io/basic-auth" {
		return nil, fmt.Errorf("secret '%s' is not of type kubernetes.io/basic-auth", secretRef.Name)
	} else if _, ok := secret.Data["username"]; !ok {
		return nil, fmt.Errorf("secret '%s' does not contain a username", secretRef.Name)
	} else if _, ok := secret.Data["password"]; !ok {
		return nil, fmt.Errorf("secret '%s' does not contain a password", secretRef.Name)
	}

	return secret, nil
}
