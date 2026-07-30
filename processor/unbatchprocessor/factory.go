package unbatchprocessor // import "go.datum.net/o11y/processor/unbatchprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"go.datum.net/o11y/processor/unbatchprocessor/internal/metadata"
)

// NewFactory returns a new factory for the Unbatch processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsProcessor(
	_ context.Context,
	_ processor.Settings,
	_ component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return newUnbatchLogs(nextConsumer), nil
}

func createMetricsProcessor(
	_ context.Context,
	_ processor.Settings,
	_ component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	return newUnbatchMetrics(nextConsumer), nil
}

func createTracesProcessor(
	_ context.Context,
	_ processor.Settings,
	_ component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	return newUnbatchTraces(nextConsumer), nil
}
