// Package natsexporter implements an exporter that publishes logs, metrics,
// and traces to a NATS JetStream stream using standard OTLP protobuf or
// OTLP JSON encoding. It is transport only: it carries whatever pdata shape
// it is given and makes no assumptions about downstream schema.
//
//go:generate go run go.opentelemetry.io/collector/cmd/mdatagen metadata.yaml
package natsexporter // import "go.datum.net/o11y/exporter/natsexporter"
