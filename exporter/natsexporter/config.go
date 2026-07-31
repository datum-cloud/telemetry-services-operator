package natsexporter // import "go.datum.net/o11y/exporter/natsexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	encodingOTLPProto = "otlp_proto"
	encodingOTLPJSON  = "otlp_json"
)

// Config defines configuration for the NATS exporter.
type Config struct {
	// URL is the NATS server URL, e.g. "nats://localhost:4222". Multiple
	// servers may be given as a comma-separated list, per the nats.go
	// connection convention.
	URL string `mapstructure:"url"`

	// Stream is the name of the JetStream stream that Logs/Metrics/Traces
	// subjects are published into. The stream must already exist, with a
	// subject filter matching the configured subjects.
	//
	// Leave empty to publish over core NATS instead of JetStream:
	// fire-and-forget, no persistence, no ack. Only safe where the
	// receiving side already durably stores what it needs (e.g. a
	// JetStream-backed hub on the other end of a leaf connection) --
	// appropriate wherever this exporter runs somewhere with no local
	// durable storage of its own.
	Stream string `mapstructure:"stream"`

	// CredentialsFile is the path to a NATS .creds file (NKey + JWT) used
	// to authenticate the connection. Leave empty to connect without
	// credentials.
	CredentialsFile string `mapstructure:"credentials_file"`

	TLS configtls.ClientConfig `mapstructure:"tls"`

	// SubjectFromAttribute, if set, names a resource attribute whose value
	// overrides the per-signal Subject for a given batch. Lets one
	// exporter instance fan out to per-tenant subjects without per-tenant
	// collector config.
	SubjectFromAttribute string `mapstructure:"subject_from_attribute"`

	Logs    SignalConfig `mapstructure:"logs"`
	Metrics SignalConfig `mapstructure:"metrics"`
	Traces  SignalConfig `mapstructure:"traces"`

	TimeoutConfig exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	RetryConfig   configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueConfig   configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
}

// SignalConfig holds signal-specific configuration for the NATS exporter.
type SignalConfig struct {
	// Subject holds the default NATS subject that messages of this signal
	// type are published to.
	Subject string `mapstructure:"subject"`

	// Encoding holds the encoding of messages for this signal type.
	// Must be "otlp_proto" or "otlp_json".
	Encoding string `mapstructure:"encoding"`
}

func (c *Config) Validate() error {
	if c.URL == "" {
		return errors.New("url must be set")
	}
	if err := c.Logs.validate("logs"); err != nil {
		return err
	}
	if err := c.Metrics.validate("metrics"); err != nil {
		return err
	}
	if err := c.Traces.validate("traces"); err != nil {
		return err
	}
	return nil
}

func (c *SignalConfig) validate(name string) error {
	switch c.Encoding {
	case encodingOTLPProto, encodingOTLPJSON:
	default:
		return fmt.Errorf("%s: unsupported encoding %q, must be %q or %q", name, c.Encoding, encodingOTLPProto, encodingOTLPJSON)
	}
	if c.Subject == "" {
		return fmt.Errorf("%s: subject must be set", name)
	}
	return nil
}
