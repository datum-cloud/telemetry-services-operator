package unbatchprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// buildLogs constructs a plog.Logs with resourceCount resources, each with
// scopesPerResource scopes, each with recordsPerScope records. Each
// resource's project_id attribute is set to its index, so tests can verify
// resource attributes survive the split correctly attached to their own
// records.
func buildLogs(resourceCount, scopesPerResource, recordsPerScope int) plog.Logs {
	ld := plog.NewLogs()
	for r := 0; r < resourceCount; r++ {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("project_id", string(rune('a'+r)))
		for s := 0; s < scopesPerResource; s++ {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName("scope")
			for i := 0; i < recordsPerScope; i++ {
				lr := sl.LogRecords().AppendEmpty()
				lr.Body().SetStr("record")
			}
		}
	}
	return ld
}

func TestUnbatchLogs_SplitsMultipleRecords(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newUnbatchLogs(sink)

	ld := buildLogs(2, 2, 2) // 2 resources * 2 scopes * 2 records = 8
	require.NoError(t, p.ConsumeLogs(context.Background(), ld))

	all := sink.AllLogs()
	require.Len(t, all, 8)
	for _, single := range all {
		assert.Equal(t, 1, single.LogRecordCount())
		assert.Equal(t, 1, single.ResourceLogs().Len())
		assert.Equal(t, 1, single.ResourceLogs().At(0).ScopeLogs().Len())
		// project_id must have travelled with its record.
		_, ok := single.ResourceLogs().At(0).Resource().Attributes().Get("project_id")
		assert.True(t, ok)
	}
}

func TestUnbatchLogs_NoOpOnSingleRecord(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newUnbatchLogs(sink)

	ld := buildLogs(1, 1, 1)
	require.NoError(t, p.ConsumeLogs(context.Background(), ld))

	all := sink.AllLogs()
	require.Len(t, all, 1)
	assert.Equal(t, 1, all[0].LogRecordCount())
}

func TestUnbatchLogs_EmptyProducesNoCalls(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newUnbatchLogs(sink)

	require.NoError(t, p.ConsumeLogs(context.Background(), plog.NewLogs()))
	assert.Empty(t, sink.AllLogs())
}

func buildTraces(resourceCount, scopesPerResource, spansPerScope int) ptrace.Traces {
	td := ptrace.NewTraces()
	for r := 0; r < resourceCount; r++ {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("project_id", string(rune('a'+r)))
		for s := 0; s < scopesPerResource; s++ {
			ss := rs.ScopeSpans().AppendEmpty()
			for i := 0; i < spansPerScope; i++ {
				span := ss.Spans().AppendEmpty()
				span.SetName("span")
			}
		}
	}
	return td
}

func TestUnbatchTraces_SplitsMultipleSpans(t *testing.T) {
	sink := new(consumertest.TracesSink)
	p := newUnbatchTraces(sink)

	td := buildTraces(2, 2, 2)
	require.NoError(t, p.ConsumeTraces(context.Background(), td))

	all := sink.AllTraces()
	require.Len(t, all, 8)
	for _, single := range all {
		assert.Equal(t, 1, single.SpanCount())
	}
}

func TestUnbatchMetrics_SplitsGaugeDataPoints(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	p := newUnbatchMetrics(sink)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("project_id", "proj-a")
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("cpu.usage")
	m.SetUnit("percent")
	m.SetEmptyGauge()
	for i := 0; i < 3; i++ {
		dp := m.Gauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(int64(i), 0)))
	}

	require.NoError(t, p.ConsumeMetrics(context.Background(), md))

	all := sink.AllMetrics()
	require.Len(t, all, 3)
	for _, single := range all {
		assert.Equal(t, 1, single.DataPointCount())
		gotM := single.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, "cpu.usage", gotM.Name())
		assert.Equal(t, "percent", gotM.Unit())
		assert.Equal(t, pmetric.MetricTypeGauge, gotM.Type())
	}
}

func TestUnbatchMetrics_SplitsSumDataPointsPreservesTemporality(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	p := newUnbatchMetrics(sink)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("requests.total")
	m.SetEmptySum()
	m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	m.Sum().SetIsMonotonic(true)
	for i := 0; i < 2; i++ {
		m.Sum().DataPoints().AppendEmpty().SetIntValue(int64(i))
	}

	require.NoError(t, p.ConsumeMetrics(context.Background(), md))

	all := sink.AllMetrics()
	require.Len(t, all, 2)
	for _, single := range all {
		gotM := single.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, pmetric.AggregationTemporalityCumulative, gotM.Sum().AggregationTemporality())
		assert.True(t, gotM.Sum().IsMonotonic())
	}
}

func TestUnbatchMetrics_EmptyProducesNoCalls(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	p := newUnbatchMetrics(sink)

	require.NoError(t, p.ConsumeMetrics(context.Background(), pmetric.NewMetrics()))
	assert.Empty(t, sink.AllMetrics())
}
