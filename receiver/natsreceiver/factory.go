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
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Logs: SignalConfig{
			Encoding: encodingOTLPProto,
		},
	}
}

func createLogsReceiver(_ context.Context, set receiver.Settings, cfg component.Config, next consumer.Logs) (receiver.Logs, error) {
	rCfg := cfg.(*Config)
	return newReceiver(rCfg, set, next)
}
