package natsreceiver // import "go.datum.net/o11y/receiver/natsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"go.datum.net/o11y/receiver/natsreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Logs: SignalConfig{
			Encoding: encodingOTLPProto,
		},
		Metrics: SignalConfig{
			Encoding: encodingOTLPProto,
		},
		Traces: SignalConfig{
			Encoding: encodingOTLPProto,
		},
	}
}

func createLogsReceiver(_ context.Context, set receiver.Settings, cfg component.Config, next consumer.Logs) (receiver.Logs, error) {
	rCfg := cfg.(*Config)
	return newLogsReceiver(rCfg, set, next)
}

func createMetricsReceiver(_ context.Context, set receiver.Settings, cfg component.Config, next consumer.Metrics) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)
	return newMetricsReceiver(rCfg, set, next)
}

func createTracesReceiver(_ context.Context, set receiver.Settings, cfg component.Config, next consumer.Traces) (receiver.Traces, error) {
	rCfg := cfg.(*Config)
	return newTracesReceiver(rCfg, set, next)
}
