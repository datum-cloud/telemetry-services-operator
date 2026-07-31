package natsreceiver // import "go.datum.net/o11y/receiver/natsreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config/configtls"
)

const (
	encodingOTLPProto = "otlp_proto"
	encodingOTLPJSON  = "otlp_json"
)

// Config defines configuration for the NATS receiver.
type Config struct {
	// URL is the NATS server URL, e.g. "nats://localhost:4222". Multiple
	// servers may be given as a comma-separated list, per the nats.go
	// connection convention.
	URL string `mapstructure:"url"`

	// Stream is the name of the JetStream stream ConsumerName's consumer
	// belongs to. Leave both Stream and ConsumerName empty to subscribe
	// over core NATS instead: fire-and-forget, no ack, no durability --
	// appropriate for a receiver running against a leaf with no local
	// JetStream of its own.
	Stream string `mapstructure:"stream"`

	// ConsumerName is the name of a pre-existing durable JetStream
	// consumer on Stream that this receiver binds to. The receiver does
	// not create the consumer -- it must already exist (see the Stream
	// field's own JetStream server or its operator). Required when Stream
	// is set; must be empty when Stream is empty.
	ConsumerName string `mapstructure:"consumer_name"`

	// CredentialsFile is the path to a NATS .creds file (NKey + JWT) used
	// to authenticate the connection. Leave empty to connect without
	// credentials.
	CredentialsFile string `mapstructure:"credentials_file"`

	TLS configtls.ClientConfig `mapstructure:"tls"`

	Logs SignalConfig `mapstructure:"logs"`
}

// SignalConfig holds signal-specific configuration for the NATS receiver.
type SignalConfig struct {
	// Subject is the NATS subject (or JetStream filter subject) this
	// receiver subscribes to.
	Subject string `mapstructure:"subject"`

	// Encoding holds the encoding of incoming messages.
	// Must be "otlp_proto" or "otlp_json".
	Encoding string `mapstructure:"encoding"`
}

func (c *Config) Validate() error {
	if c.URL == "" {
		return errors.New("url must be set")
	}
	if c.Stream != "" && c.ConsumerName == "" {
		return errors.New("consumer_name must be set when stream is set")
	}
	if c.Stream == "" && c.ConsumerName != "" {
		return errors.New("stream must be set when consumer_name is set")
	}
	return c.Logs.validate("logs")
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
