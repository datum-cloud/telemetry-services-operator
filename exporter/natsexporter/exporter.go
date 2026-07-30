package natsexporter // import "go.datum.net/o11y/exporter/natsexporter"

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type natsExporter struct {
	cfg *Config
	set exporter.Settings

	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
	tracesMarshaler  ptrace.Marshaler

	conn *nats.Conn
	js   jetstream.JetStream
}

func newExporter(cfg *Config, set exporter.Settings) (*natsExporter, error) {
	logsMarshaler, err := newLogsMarshaler(cfg.Logs.Encoding)
	if err != nil {
		return nil, err
	}
	metricsMarshaler, err := newMetricsMarshaler(cfg.Metrics.Encoding)
	if err != nil {
		return nil, err
	}
	tracesMarshaler, err := newTracesMarshaler(cfg.Traces.Encoding)
	if err != nil {
		return nil, err
	}
	return &natsExporter{
		cfg:              cfg,
		set:              set,
		logsMarshaler:    logsMarshaler,
		metricsMarshaler: metricsMarshaler,
		tracesMarshaler:  tracesMarshaler,
	}, nil
}

func (e *natsExporter) start(ctx context.Context, _ component.Host) error {
	opts := []nats.Option{nats.Name(e.set.ID.String())}
	if e.cfg.CredentialsFile != "" {
		opts = append(opts, nats.UserCredentials(e.cfg.CredentialsFile))
	}

	tlsCfg, err := e.cfg.TLS.LoadTLSConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading tls config: %w", err)
	}
	if tlsCfg != nil {
		opts = append(opts, nats.Secure(tlsCfg))
	}

	conn, err := nats.Connect(e.cfg.URL, opts...)
	if err != nil {
		return fmt.Errorf("connecting to nats: %w", err)
	}
	e.conn = conn

	switch e.cfg.Stream {
	case "":
		return e.startCoreNATS()
	default:
		return e.startJetStream(conn)
	}
}

func (e *natsExporter) startCoreNATS() error {
	return nil
}

func (e *natsExporter) startJetStream(conn *nats.Conn) error {
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("creating jetstream context: %w", err)
	}
	e.js = js
	return nil
}

func (e *natsExporter) shutdown(context.Context) error {
	if e.conn != nil {
		return e.conn.Drain()
	}
	return nil
}

// subjectFor returns the configured subject for a signal, overridden by
// cfg.SubjectFromAttribute when it names an attribute present on attrs.
// attrs is the Resource of the batch's first ResourceLogs/Metrics/Spans
// entry; callers with multiple resources per batch (e.g. an exporter fed
// directly rather than via unbatchprocessor) only get attribute-based
// routing for the first resource in the batch.
func (e *natsExporter) subjectFor(signal SignalConfig, attrs pcommon.Map) string {
	if e.cfg.SubjectFromAttribute != "" {
		if v, ok := attrs.Get(e.cfg.SubjectFromAttribute); ok {
			return v.AsString()
		}
	}
	return signal.Subject
}

func (e *natsExporter) publish(ctx context.Context, subject string, payload []byte) error {
	if e.js != nil {
		_, err := e.js.Publish(ctx, subject, payload)
		return err
	}
	return e.conn.Publish(subject, payload)
}

func (e *natsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	subject := e.cfg.Logs.Subject
	if ld.ResourceLogs().Len() > 0 {
		subject = e.subjectFor(e.cfg.Logs, ld.ResourceLogs().At(0).Resource().Attributes())
	}
	payload, err := e.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return fmt.Errorf("marshaling logs: %w", err)
	}
	return e.publish(ctx, subject, payload)
}

func (e *natsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	subject := e.cfg.Metrics.Subject
	if md.ResourceMetrics().Len() > 0 {
		subject = e.subjectFor(e.cfg.Metrics, md.ResourceMetrics().At(0).Resource().Attributes())
	}
	payload, err := e.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return fmt.Errorf("marshaling metrics: %w", err)
	}
	return e.publish(ctx, subject, payload)
}

func (e *natsExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	subject := e.cfg.Traces.Subject
	if td.ResourceSpans().Len() > 0 {
		subject = e.subjectFor(e.cfg.Traces, td.ResourceSpans().At(0).Resource().Attributes())
	}
	payload, err := e.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return fmt.Errorf("marshaling traces: %w", err)
	}
	return e.publish(ctx, subject, payload)
}

func newLogsMarshaler(encoding string) (plog.Marshaler, error) {
	switch encoding {
	case encodingOTLPProto:
		return &plog.ProtoMarshaler{}, nil
	case encodingOTLPJSON:
		return &plog.JSONMarshaler{}, nil
	default:
		return nil, fmt.Errorf("unsupported logs encoding %q", encoding)
	}
}

func newMetricsMarshaler(encoding string) (pmetric.Marshaler, error) {
	switch encoding {
	case encodingOTLPProto:
		return &pmetric.ProtoMarshaler{}, nil
	case encodingOTLPJSON:
		return &pmetric.JSONMarshaler{}, nil
	default:
		return nil, fmt.Errorf("unsupported metrics encoding %q", encoding)
	}
}

func newTracesMarshaler(encoding string) (ptrace.Marshaler, error) {
	switch encoding {
	case encodingOTLPProto:
		return &ptrace.ProtoMarshaler{}, nil
	case encodingOTLPJSON:
		return &ptrace.JSONMarshaler{}, nil
	default:
		return nil, fmt.Errorf("unsupported traces encoding %q", encoding)
	}
}
