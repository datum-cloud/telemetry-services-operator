package natsexporter // import "go.datum.net/o11y/exporter/natsexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"go.datum.net/o11y/exporter/natsexporter/internal/metadata"
)

const (
	defaultLogsSubject    = "otlp.logs"
	defaultMetricsSubject = "otlp.metrics"
	defaultTracesSubject  = "otlp.traces"
	defaultEncoding       = encodingOTLPProto
)

// NewFactory creates a factory for the NATS exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig: exporterhelper.NewDefaultTimeoutConfig(),
		RetryConfig:   configretry.NewDefaultBackOffConfig(),
		QueueConfig:   configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		Logs:          SignalConfig{Subject: defaultLogsSubject, Encoding: defaultEncoding},
		Metrics:       SignalConfig{Subject: defaultMetricsSubject, Encoding: defaultEncoding},
		Traces:        SignalConfig{Subject: defaultTracesSubject, Encoding: defaultEncoding},
	}
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	oCfg := cfg.(*Config)
	exp, err := newExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(ctx, set, cfg,
		exp.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutConfig),
		exporterhelper.WithRetry(oCfg.RetryConfig),
		exporterhelper.WithQueue(oCfg.QueueConfig),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oCfg := cfg.(*Config)
	exp, err := newExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetrics(ctx, set, cfg,
		exp.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutConfig),
		exporterhelper.WithRetry(oCfg.RetryConfig),
		exporterhelper.WithQueue(oCfg.QueueConfig),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
	)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	oCfg := cfg.(*Config)
	exp, err := newExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTraces(ctx, set, cfg,
		exp.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutConfig),
		exporterhelper.WithRetry(oCfg.RetryConfig),
		exporterhelper.WithQueue(oCfg.QueueConfig),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
	)
}
