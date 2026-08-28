CREATE TABLE IF NOT EXISTS logs
(
    -- Event time: when the event occurred, OTLP time_unix_nano. May be 0 or
    -- unreliable for sources that don't set it, so it is not the query key.
    Timestamp DateTime64(9),
    -- Observed time: when the OTel Collector received the record, frozen at
    -- receipt and carried unchanged through the pipeline. This is the primary
    -- query/partition/order key: `datumctl logs --since`, the query layer's
    -- time window, and the partition/order key all use observed time so that
    -- a backlog drain or an unreliable source clock never shifts the sort or
    -- pruning window. Both columns follow the OpenTelemetry Logs data model.
    ObservedTimestamp DateTime64(9),
    TraceId String,
    SpanId String,
    TraceFlags UInt8,
    SeverityText LowCardinality(String),
    SeverityNumber UInt8,
    ServiceName LowCardinality(String),
    Body String,
    ResourceSchemaUrl LowCardinality(String),
    ResourceAttributes Map(String, String),
    ScopeSchemaUrl LowCardinality(String),
    ScopeName String,
    ScopeVersion LowCardinality(String),
    ScopeAttributes Map(String, String),
    LogAttributes Map(String, String),
    EventName String,
    ProjectId String MATERIALIZED ResourceAttributes['milo.project.id']
)
ENGINE = MergeTree
-- Monthly partition on observed time keeps the partition count bounded as the
-- tenant count grows (N projects x days would explode with a daily partition).
-- Retention is intentionally NOT declared here: TTL policy (delete and
-- cold-tier deadlines, per the internal-vs-tenant tiers) is a service-provider
-- decision supplied by the deployment repo via ClickHouse custom settings or a
-- follow-up migration, not a value baked into a shared schema.
PARTITION BY toYYYYMM(ObservedTimestamp)
ORDER BY (ProjectId, ObservedTimestamp, ServiceName);

-- Authorization.
--
-- Two users share this table: ops (full read, provider-defined) and the
-- query-serving identity (the HTTP query layer, whose ClickHouse identity is
-- certificate-mapped in the provider's users.d/ssl_auth.xml). That identity's
-- literal username varies per deployment, so it is injected at migration
-- render time as {{QUERYAPI_USER}} (see clickhouse-migrate's
-- CLICKHOUSE_QUERYAPI_USER env var). It is granted SELECT only, scoped to
-- this database, plus the two privileges ClickHouse requires before a user
-- may read or set a custom setting -- without these, every query fails with
-- "unknown setting" the moment it tries to set telemetry_project_id -- and is
-- row-scoped by a policy keyed on that per-query custom setting the query
-- layer sets (a query without it matches no rows).

GRANT SELECT ON logs TO `{{QUERYAPI_USER}}`;
GRANT settings_allow_custom_setting_read, settings_allow_custom_setting_write ON *.* TO `{{QUERYAPI_USER}}`;

CREATE ROW POLICY IF NOT EXISTS queryapi_project_isolation
ON logs
FOR SELECT
USING ProjectId = getSetting('telemetry_project_id')
TO `{{QUERYAPI_USER}}`;
