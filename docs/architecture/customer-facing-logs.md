# Customer-Facing Logs

Status: Draft
Scope (v1): AI Edge (HTTPProxy + WAF) logs only

## Motivation

Datum platform services emit operational signals — request logs, security
events, control-plane activity — that customers need visibility into for
debugging, compliance, and security investigation. Today there is no
customer-facing query surface for these logs. Customers running workloads on
AI Edge (Datum's HTTP proxy + WAF product) cannot answer basic questions
like "show me 5xx responses for my proxy in the last hour" or "which
requests did the WAF block."

This design defines a project-scoped, multi-tenant logs pipeline with a
Loki-compatible query API. AI Edge is the v1 scope: it produces high-volume
access logs and WAF events that are the most acute customer need, and its
log shape exercises every layer of the design without depending on
control-plane audit-log work that lives elsewhere.

## Goals (v1)

- Customers can query AI Edge access logs and WAF events for their project
  through Grafana, LogCLI, and any Loki-compatible client.
- All logs are tenant-isolated at storage and query time; cross-tenant
  reads are structurally impossible.
- Log schemas are declared once by the producing service and surface
  automatically as catalog metadata (resource types, label vocabulary, log
  definitions).
- 7-day default retention for operational logs, with a longer default for
  any log marked as `audit` category.

## Non-Goals (v1)

- Control-plane audit log surface (covered by `milo-os/activity`; integrated
  later via a shared catalog).
- Customer-configurable log export (`LogSource` in `ExportPolicy`) —
  deferred to a follow-on enhancement.
- Body-content redaction via regex; v1 redacts at attribute level only.
- Log-based metrics and alerting derived from log streams.

## Layers

### 1. Service Declaration

Services declare what they emit in their `ServiceConfiguration`
(`services.miloapis.com/v1alpha1`). Two fields participate:

- `spec.monitoredResourceTypes[]` — already fans out to
  `billing.MonitoredResourceType`; now also fans out to a new
  `telemetry.MonitoredResourceType`.
- `spec.logs[]` (new) — fans out to `telemetry.LogDefinition`.

AI Edge declaration:

```yaml
apiVersion: services.miloapis.com/v1alpha1
kind: ServiceConfiguration
metadata:
  name: networking-datumapis-com
spec:
  serviceRef:
    name: networking-datumapis-com
  phase: Published
  monitoredResourceTypes:
    - resourceTypeName: networking.datumapis.com/HTTPProxy
      displayName: HTTP Proxy
      gvk:
        group: networking.datumapis.com
        kind: HTTPProxy
      labels:
        - name: resource.name
          description: Name of the HTTPProxy instance.
        - name: resource.namespace
          description: Project namespace the HTTPProxy belongs to.
        - name: hostname
          description: Hostname the request was received on.
  logs:
    - logID: networking.datumapis.com/httpproxy-access
      displayName: HTTP Proxy Access Log
      description: One entry per HTTP request handled by the proxy.
      monitoredResourceType: networking.datumapis.com/HTTPProxy
      entrySchema:
        - name: http.request.method
          description: HTTP method (GET, POST, etc).
        - name: http.response.status_code
          description: HTTP response status returned to the client.
        - name: url.path
          description: Request path.
        - name: client.address
          description: Client IP.
        - name: http.request.duration_ms
          description: Request duration in milliseconds.
      destinations:
        - audience: tenant
        - audience: platform
      categoryGroups: [allLogs]

    - logID: networking.datumapis.com/httpproxy-waf
      displayName: HTTP Proxy WAF Event Log
      description: One entry per WAF rule evaluation that matched or blocked.
      monitoredResourceType: networking.datumapis.com/HTTPProxy
      entrySchema:
        - name: waf.rule.id
          description: Identifier of the WAF rule that matched.
        - name: waf.action
          description: Action taken — block, log, challenge.
        - name: waf.severity
          description: Severity classification of the matched rule.
        - name: client.address
          description: Client IP.
      destinations:
        - audience: tenant
        - audience: platform
      categoryGroups: [allLogs, audit]
```

### 2. Platform Catalog

The services operator (`milo-os/telemetry`) owns two new CRDs that the
`ServiceConfiguration` controller fans out into.

`telemetry.MonitoredResourceType` — instance-identifying label vocabulary
for a resource Kind. Parallel to `billing.MonitoredResourceType`:

```yaml
apiVersion: telemetry.miloapis.com/v1alpha1
kind: MonitoredResourceType
metadata:
  name: networking-datumapis-com-httpproxy
spec:
  resourceTypeName: networking.datumapis.com/HTTPProxy
  phase: Published
  displayName: HTTP Proxy
  gvk:
    group: networking.datumapis.com
    kind: HTTPProxy
  labels:
    - name: resource.name
    - name: resource.namespace
    - name: hostname
```

`LogDefinition` — the log type catalog entry; references
`MonitoredResourceType` by `resourceTypeName`:

```yaml
apiVersion: telemetry.miloapis.com/v1alpha1
kind: LogDefinition
metadata:
  name: networking-datumapis-com-httpproxy-access
spec:
  logID: networking.datumapis.com/httpproxy-access
  phase: Published
  displayName: HTTP Proxy Access Log
  monitoredResourceType: networking.datumapis.com/HTTPProxy
  entrySchema:
    - name: http.request.method
    - name: http.response.status_code
    - name: url.path
    - name: client.address
    - name: http.request.duration_ms
  destinations:
    - audience: tenant
    - audience: platform
  categoryGroups: [allLogs]
```

Both CRDs are server-managed: the `ServiceConfiguration` controller is the
sole writer. Customers read them via standard list/get to populate UIs and
discover available log types.

### 3. Ingestion Pipeline

AI Edge data-plane components (Envoy + WAF sidecar) emit logs over OTLP to
a regional OTel Collector gateway.

Gateway responsibilities:

1. Receive OTLP log records.
2. Stamp `cloud.account.id` (Milo project ID) immutably from the caller's
   workload identity — customers cannot override.
3. Look up the declared `MonitoredResourceType` for the entry's
   `resource_type` and validate that emitted resource attributes are a
   subset of the declared label vocabulary. Reject undeclared labels.
4. Derive `tenant_id` from `cloud.account.id`.
5. Write to ClickHouse via the `clickhouse` exporter.

Services are responsible for stamping the instance-identifying labels
(e.g. `resource.name`, `resource.namespace`, `hostname`). The gateway
enforces the vocabulary; it does not inject instance identity.

### 4. Storage

Shared ClickHouse `platform_logs` table, OTel-aligned schema, `tenant_id`
first in `ORDER BY` and partition key:

```sql
CREATE TABLE platform_logs (
    tenant_id           UInt32,
    timestamp           UInt64,
    observed_timestamp  UInt64,
    severity_number     UInt8,
    severity_text       LowCardinality(String),
    body                String,
    log_id              LowCardinality(String),
    resource_type       LowCardinality(String),
    attributes_string   Map(String, String),
    resources_string    Map(String, String),
    trace_id            String,
    span_id             String
)
ENGINE = MergeTree()
PARTITION BY (tenant_id, toYYYYMM(toDateTime(timestamp / 1e9)))
ORDER BY (tenant_id, log_id, timestamp)
TTL toDateTime(timestamp / 1e9) + INTERVAL 7 DAY DELETE;
```

`log_id` and `resource_type` are promoted to top-level columns: both are
low-cardinality and appear in nearly every query's filter clause.

Per-tenant retention overrides are applied via per-row `_row_ttl`
attribute set by the gateway based on the log's `categoryGroups` and the
tenant's retention policy (see Retention below).

### 5. Query API — Loki-Compatible, Project-Scoped

Customer query surface is a Loki-compatible HTTP API exposed under the
project's telemetry namespace:

```
GET  /projects/{project}/telemetry/loki/api/v1/query
GET  /projects/{project}/telemetry/loki/api/v1/query_range
GET  /projects/{project}/telemetry/loki/api/v1/labels
GET  /projects/{project}/telemetry/loki/api/v1/label/{name}/values
GET  /projects/{project}/telemetry/loki/api/v1/series
GET  /projects/{project}/telemetry/loki/api/v1/tail
```

The Milo gateway resolves `{project}` to a `tenant_id` and enforces IAM
before the request reaches the Loki handler. The handler itself is a pure
query layer:

- Parses LogQL.
- Translates to ClickHouse SQL: stream selectors → `resources_string` map
  lookups; line filters → `body LIKE` / full-text; parsed field filters →
  `attributes_string` lookups.
- Executes with `tenant_id` already injected from URL context.
- Serialises results in Loki's response format.

`X-Scope-OrgID` sent by Grafana is ignored — the project in the URL is
authoritative.

Label and series discovery is served from the `MonitoredResourceType`
catalog rather than from ClickHouse, so discovery works on empty projects
and Grafana's stream-selector UI populates correctly on first open.

Grafana datasource configuration: base URL
`https://api.datum.net/projects/{project}/telemetry/`, type Loki, no
custom plugin.

A secondary `LogQuery` virtual resource (Kubernetes-native, modelled on
`AuditLogQuery` in `milo-os/activity`) is retained for kubectl-native and
GitOps workflows. It shares the same LogQL → SQL translation layer.

### 6. Access Control

- Milo IAM gates access at the project boundary via standard Kubernetes
  RBAC on the project's telemetry endpoint.
- `LogDefinition.spec.categoryGroups` provides a secondary access
  dimension: `audit` requires a distinct permission from `allLogs`,
  matching GCP's `roles/logging.viewAccessor` pattern scoped to a log
  view. The query layer filters out log IDs the caller cannot access
  before executing the SQL.

## Cross-Cutting Concerns

### Retention

Fixed tiered defaults; no free-form per-project retention in v1.

| Category Group | Default Retention | Disable-able |
|---|---|---|
| `allLogs`      | 7 days    | Yes (opt-in collection) |
| `audit`        | 400 days  | No (compliance signal)  |

Paid retention overrides are applied per category group on a project, not
per log ID. Implemented as a TTL adjustment column populated by the
gateway at write time so existing rows are not rewritten when overrides
change.

### Ingestion Quota

A new `telemetry.miloapis.com/LogIngestionQuota` resource integrates with
the standard Milo quota system. Quota is dimensioned by
`(project, category_group)` in bytes/second. On exceed:

- Gateway returns 429 with `Retry-After`.
- A per-tenant `telemetry_ingestion_dropped_bytes_total` counter is
  exposed via the same Loki API so customers can see drops.
- No silent drops.

### Default Enablement

- `allLogs` collection is opt-in per project via a `LogCollectionPolicy`
  resource. Customers don't get surprise bills from log volume tracking
  workload activity they didn't request.
- `audit` category is on by default and not disable-able. Volume is
  bounded by control-plane API traffic, not workload activity.

For v1 (AI Edge only): proxy access logs default off, WAF events default
on (they fall into both `allLogs` and `audit` and the volume is bounded
by request rate × match rate, not full request rate).

### Redaction

- Platform-managed allowlist of attribute keys always dropped or hashed
  at the gateway (`*.token`, `*.password`, `authorization`, ...).
- Customer-configurable `LogRedactionPolicy` resource — attribute-level
  drop/hash rules only.
- Body content is **not** redacted in v1. Documented as a constraint;
  services are pushed to put structured data in attributes.

## Fan-Out Summary

```
ServiceConfiguration
  spec.monitoredResourceTypes[]  →  billing.MonitoredResourceType   (existing)
                                 →  telemetry.MonitoredResourceType (new)
  spec.logs[]                    →  telemetry.LogDefinition          (new)
```

## v1 Delivery Slice

In dependency order:

1. CRDs: `MonitoredResourceType`, `LogDefinition`,
   `LogCollectionPolicy`, `LogIngestionQuota`, `LogRedactionPolicy`,
   `LogQuery`.
2. Fan-out controllers in this operator for the first three.
3. ClickHouse `platform_logs` table and OTel Collector gateway with
   tenant stamping, label-vocabulary validation, and quota enforcement.
4. AI Edge data-plane integration: Envoy access log + WAF event OTLP
   exporters; `ServiceConfiguration` for `networking-datumapis-com` with
   the two log definitions.
5. Loki API handler (`/projects/{project}/telemetry/loki/api/v1/...`)
   backed by a LogQL → SQL translator.
6. Catalog-backed labels/series discovery.
7. Grafana datasource documentation.

## Open Questions

- Live tail backend: ClickHouse polling vs. a separate Kafka topic
  consumed by the tail handler. Polling is simpler; Kafka is lower
  latency. Likely poll for v1.
- Whether `LogCollectionPolicy` is project-scoped or finer-grained (per
  `HTTPProxy`). Project-scoped is the simpler v1; finer granularity is a
  future enhancement once we see usage patterns.
- Loki LogQL feature subset for v1: instant queries, range queries,
  line filters, label filters are required; metric queries
  (`rate`, `sum by`, ...) likely deferred to v2.
- How the catalog-backed label discovery handles tenant-specific label
  values (e.g. the set of `resource.name` values that actually exist in
  the project). Likely a hybrid: label names from catalog, values from
  ClickHouse with a short cache.
