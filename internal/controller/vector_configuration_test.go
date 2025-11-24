package controller

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"go.datum.net/telemetry-services-operator/api/v1alpha1"
)

func TestCreateVectorConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		exportPolicy *v1alpha1.ExportPolicy
		assert       func(t *testing.T, ep *v1alpha1.ExportPolicy, vectorConfig map[string]any)
	}{
		{
			name:         "project filter is present when no filters are specified",
			exportPolicy: newExportPolicy(),
			assert: func(t *testing.T, ep *v1alpha1.ExportPolicy, vectorConfig map[string]any) {
				vectorSources := vectorConfig["sources"].(map[string]any)

				if assert.Len(t, vectorSources, 1) {
					sources := slices.Collect(maps.Keys(vectorSources))

					source := vectorSources[sources[0]].(map[string]any)
					if assert.Contains(t, source, "query") && assert.Contains(t, source["query"], "match[]") {
						query := source["query"].(map[string]any)
						matchers := query["match[]"].([]string)
						assert.Contains(t, matchers[0], `resourcemanager_datumapis_com_project_name="test-project"`)
					}
				}
			},
		},
		{
			name: "project filter is present when filters are specified",
			exportPolicy: newExportPolicy(func(ep *v1alpha1.ExportPolicy) {
				ep.Spec.Sources[0].Metrics.MetricsQL = `{job="my-job"}`
			}),
			assert: func(t *testing.T, ep *v1alpha1.ExportPolicy, vectorConfig map[string]any) {
				vectorSources := vectorConfig["sources"].(map[string]any)

				if assert.Len(t, vectorSources, 1) {
					sources := slices.Collect(maps.Keys(vectorSources))

					source := vectorSources[sources[0]].(map[string]any)
					if assert.Contains(t, source, "query") && assert.Contains(t, source["query"], "match[]") {
						query := source["query"].(map[string]any)
						matchers := query["match[]"].([]string)
						assert.Contains(t, matchers[0], `resourcemanager_datumapis_com_project_name="test-project"`)
					}
				}
			},
		},
		{
			name: "matching source and sink name produces unique component names",
			exportPolicy: newExportPolicy(func(ep *v1alpha1.ExportPolicy) {
				ep.Spec.Sinks = []v1alpha1.TelemetrySink{
					{
						Name:    "source",
						Sources: []string{"source"},
						Target: &v1alpha1.SinkTarget{
							PrometheusRemoteWrite: &v1alpha1.PrometheusRemoteWriteSink{},
						},
					},
				}
			}),
			assert: func(t *testing.T, ep *v1alpha1.ExportPolicy, vectorConfig map[string]any) {
				sourceComponentNames := slices.Collect(maps.Keys(vectorConfig["sources"].(map[string]any)))
				sinkComponentNames := slices.Collect(maps.Keys(vectorConfig["sinks"].(map[string]any)))

				for _, sourceName := range sourceComponentNames {
					assert.NotContains(t, sinkComponentNames, sourceName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &ExportPolicyReconciler{}

			vectorConfig := reconciler.createVectorConfiguration(context.Background(), "test-project", nil, tt.exportPolicy)

			tt.assert(t, tt.exportPolicy, vectorConfig)
		})
	}
}

func newExportPolicy(opts ...func(*v1alpha1.ExportPolicy)) *v1alpha1.ExportPolicy {
	p := &v1alpha1.ExportPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID:       uuid.NewUUID(),
			Namespace: "test-namespace",
			Name:      "test-exportpolicy",
		},
		Spec: v1alpha1.ExportPolicySpec{
			Sources: []v1alpha1.TelemetrySource{
				{
					Name: "source",
					Metrics: &v1alpha1.MetricSource{
						MetricsQL: "{}",
					},
				},
			},
			Sinks: []v1alpha1.TelemetrySink{
				{
					Name:    "sink",
					Sources: []string{"source"},
					Target: &v1alpha1.SinkTarget{
						PrometheusRemoteWrite: &v1alpha1.PrometheusRemoteWriteSink{},
					},
				},
			},
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}
