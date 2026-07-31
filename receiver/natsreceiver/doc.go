// Package natsreceiver implements an OpenTelemetry Collector receiver that
// subscribes to a NATS subject and forwards decoded OTLP payloads into the
// pipeline. Supports both core NATS (fire-and-forget) and JetStream
// (binding to a pre-existing durable consumer, with automatic re-pull on a
// missed heartbeat -- including after a JetStream consumer leader
// election) transport modes.
package natsreceiver // import "go.datum.net/o11y/receiver/natsreceiver"
