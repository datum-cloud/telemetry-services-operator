package natsreceiver

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// startTestServer starts an in-process NATS server with JetStream enabled
// on a random port, and tears it down at the end of the test.
func startTestServer(t *testing.T) *natsserver.Server {
	t.Helper()
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	srv, err := natsserver.NewServer(opts)
	require.NoError(t, err)
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server did not become ready")
	}
	t.Cleanup(srv.Shutdown)
	return srv
}

// createTestStream connects to srv and creates a JetStream stream covering
// the given subjects, returning a JetStream context usable for consuming
// published messages back out in tests.
func createTestStream(t *testing.T, srv *natsserver.Server, stream string, subjects ...string) jetstream.JetStream {
	t.Helper()
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     stream,
		Subjects: subjects,
	})
	require.NoError(t, err)
	return js
}

func createTestConsumer(t *testing.T, srv *natsserver.Server, stream, consumerName string, subjects ...string) {
	t.Helper()
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = js.CreateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: subjects[0],
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	require.NoError(t, err)
}

func TestReceiver_JetStreamMode_DeliversLogs(t *testing.T) {
	srv := startTestServer(t)
	createTestStream(t, srv, "otlp", "otlp.>")
	createTestConsumer(t, srv, "otlp", "test-consumer", "otlp.logs")

	cfg := &Config{
		URL:          srv.ClientURL(),
		Stream:       "otlp",
		ConsumerName: "test-consumer",
		Logs:         SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"},
	}
	cfg.TLS.Insecure = true
	sink := new(consumertest.LogsSink)
	set := receivertest.NewNopSettings(receivertest.NopType)
	rcv, err := newReceiver(cfg, set, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(t.Context())) })

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("hello")
	payload, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)
	require.NoError(t, nc.Publish("otlp.logs", payload))

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 1
	}, 5*time.Second, 50*time.Millisecond)
	assert := require.New(t)
	assert.Equal("hello", sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}

func TestReceiver_CoreNATSMode_DeliversLogs(t *testing.T) {
	srv := startTestServer(t)

	cfg := &Config{
		URL:  srv.ClientURL(),
		Logs: SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"},
	}
	cfg.TLS.Insecure = true
	sink := new(consumertest.LogsSink)
	set := receivertest.NewNopSettings(receivertest.NopType)
	rcv, err := newReceiver(cfg, set, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(t.Context())) })

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("core-nats-hello")
	payload, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)
	require.NoError(t, nc.Publish("otlp.logs", payload))

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 1
	}, 5*time.Second, 50*time.Millisecond)
}

// TestReceiver_JetStreamMode_NaksOnConsumeError proves the fix for the
// silent-data-loss bug: if the downstream consumer fails, handleJetStream
// must Nak (not Ack) so the durable consumer redelivers, instead of the
// message being acked and lost forever.
func TestReceiver_JetStreamMode_NaksOnConsumeError(t *testing.T) {
	srv := startTestServer(t)
	createTestStream(t, srv, "otlp", "otlp.>")
	createTestConsumer(t, srv, "otlp", "test-consumer", "otlp.logs")

	cfg := &Config{
		URL:          srv.ClientURL(),
		Stream:       "otlp",
		ConsumerName: "test-consumer",
		Logs:         SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"},
	}
	cfg.TLS.Insecure = true

	var attempts atomic.Int32
	// Always fails, simulating a downstream exporter (e.g. clickhouseexporter)
	// that has exhausted its own retries. Counts attempts so the test can
	// tell whether the message was redelivered.
	failingConsumer, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		attempts.Add(1)
		return errors.New("simulated downstream export failure")
	})
	require.NoError(t, err)

	set := receivertest.NewNopSettings(receivertest.NopType)
	rcv, err := newReceiver(cfg, set, failingConsumer)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(t.Context())) })

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("will-fail")
	payload, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)
	require.NoError(t, nc.Publish("otlp.logs", payload))

	// With the pre-fix behavior (unconditional Ack), attempts would stay at
	// 1 forever. With Nak on failure, JetStream redelivers the message.
	require.Eventually(t, func() bool {
		return attempts.Load() > 1
	}, 5*time.Second, 50*time.Millisecond, "message was not redelivered after a consume failure -- Ack must be gated on delivery success")
}

// fakeJSMsg is a minimal jetstream.Msg mock that records which ack-flavor
// method handleJetStream calls, so the retryable/non-retryable branches can
// be tested without a live NATS server.
type fakeJSMsg struct {
	data []byte

	acked          bool
	nakCalled      bool
	nakDelayCalled time.Duration
	nakDelayCount  int
}

func (m *fakeJSMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }
func (m *fakeJSMsg) Data() []byte                              { return m.data }
func (m *fakeJSMsg) Headers() nats.Header                      { return nil }
func (m *fakeJSMsg) Subject() string                           { return "test" }
func (m *fakeJSMsg) Reply() string                             { return "" }
func (m *fakeJSMsg) Ack() error                                { m.acked = true; return nil }
func (m *fakeJSMsg) DoubleAck(context.Context) error           { m.acked = true; return nil }
func (m *fakeJSMsg) Nak() error                                { m.nakCalled = true; return nil }
func (m *fakeJSMsg) NakWithDelay(delay time.Duration) error {
	m.nakDelayCount++
	m.nakDelayCalled = delay
	return nil
}
func (m *fakeJSMsg) InProgress() error                  { return nil }
func (m *fakeJSMsg) Term() error                        { return nil }
func (m *fakeJSMsg) TermWithReason(reason string) error { return nil }

// TestHandleJetStream_UnmarshalError_AcksNotNaks proves the fix for the
// poison-pill loop: a malformed payload that will never unmarshal
// successfully must be Acked (dropped), not Nak'd, so it isn't redelivered
// into an infinite, backoff-free retry loop.
func TestHandleJetStream_UnmarshalError_AcksNotNaks(t *testing.T) {
	cfg := &Config{Logs: SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"}}
	set := receivertest.NewNopSettings(receivertest.NopType)
	sink := new(consumertest.LogsSink)
	rcv, err := newReceiver(cfg, set, sink)
	require.NoError(t, err)

	msg := &fakeJSMsg{data: []byte("not a valid otlp proto payload")}
	rcv.handleJetStream(msg)

	require.True(t, msg.acked, "unprocessable payload must be Acked (dropped), not redelivered")
	require.False(t, msg.nakCalled)
	require.Zero(t, msg.nakDelayCount)
	require.Equal(t, 0, sink.LogRecordCount())
}

// TestHandleJetStream_ExportError_NaksWithDelay proves the fix for the
// instant-redelivery tight loop: a retryable downstream export failure must
// use NakWithDelay (not a bare Nak, which ignores AckWait/Backoff and
// redelivers instantly).
func TestHandleJetStream_ExportError_NaksWithDelay(t *testing.T) {
	cfg := &Config{Logs: SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"}}
	set := receivertest.NewNopSettings(receivertest.NopType)
	failingConsumer, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		return errors.New("simulated downstream export failure")
	})
	require.NoError(t, err)
	rcv, err := newReceiver(cfg, set, failingConsumer)
	require.NoError(t, err)

	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("hi")
	payload, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)

	msg := &fakeJSMsg{data: payload}
	rcv.handleJetStream(msg)

	require.False(t, msg.acked, "a retryable export failure must not be acked")
	require.False(t, msg.nakCalled, "must use NakWithDelay, not a bare Nak (instant, backoff-free redelivery)")
	require.Equal(t, 1, msg.nakDelayCount)
	require.Equal(t, nakRedeliveryDelay, msg.nakDelayCalled)
}

func TestReceiver_OTLPJSONEncoding(t *testing.T) {
	srv := startTestServer(t)

	cfg := &Config{
		URL:  srv.ClientURL(),
		Logs: SignalConfig{Subject: "otlp.logs", Encoding: "otlp_json"},
	}
	cfg.TLS.Insecure = true
	sink := new(consumertest.LogsSink)
	set := receivertest.NewNopSettings(receivertest.NopType)
	rcv, err := newReceiver(cfg, set, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(t.Context())) })

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("json-hello")
	payload, err := (&plog.JSONMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)
	require.NoError(t, nc.Publish("otlp.logs", payload))

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 1
	}, 5*time.Second, 50*time.Millisecond)
}

// TestReceiver_LastDeliveryRace_ConcurrentDeliverAndScrape proves the fix
// for the lastDelivery data race: deliver() (called from NATS client
// goroutines) writes lastDelivery while the metrics SDK's Collect (called
// from a separate goroutine on scrape) reads it via the gauge callback.
// Before the fix (plain time.Time with no synchronization), `go test -race`
// would flag this as a race; run with -race to verify.
func TestReceiver_LastDeliveryRace_ConcurrentDeliverAndScrape(t *testing.T) {
	srv := startTestServer(t)
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	cfg := &Config{URL: srv.ClientURL(), Logs: SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"}}
	cfg.TLS.Insecure = true
	set := receivertest.NewNopSettings(receivertest.NopType)
	set.TelemetrySettings.MeterProvider = mp

	sink := new(consumertest.LogsSink)
	rcv, err := newReceiver(cfg, set, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(t.Context())) })

	ld2 := plog.NewLogs()
	rl2 := ld2.ResourceLogs().AppendEmpty()
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("race")
	racePayload, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld2)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			// Simulates the NATS client's delivery callback goroutine
			// writing lastDelivery on every successful delivery.
			_ = rcv.deliver(t.Context(), racePayload)
		}
	}()

	var rm metricdata.ResourceMetrics
	for i := 0; i < 200; i++ {
		// Simulates the OTel SDK's own collection goroutine reading
		// lastDelivery via the gauge callback on scrape.
		_ = reader.Collect(t.Context(), &rm)
	}
	<-done
}
