# queryapi

Tenant-scoped log and metric query API over ClickHouse, serving `datumctl` and
the customer cloud portal. See [`openapi.yaml`](openapi.yaml) for the contract.

## Storage backends

| `--storage` | Behaviour |
| --- | --- |
| `fake` (default) | Deterministic synthetic logs, a pure function of the query's time range. Same range always returns the same rows. |
| `clickhouse` | Serves log queries and label/stream discovery against the `o11y.logs` schema via LogQL-to-SQL translation. `/readyz` pings the backend. |

Metric (Prometheus-shaped) endpoints return 501 on both backends.

## Flags

| Flag | Default |
| --- | --- |
| `--addr` | `:8080` |
| `--storage` | `fake` |
| `--fake-rate` | `2` (lines/sec) |
| `--query-timeout` | `30s` |
| `--default-limit` | `100` |
| `--max-limit` | `5000` |
| `--read-header-timeout` | `5s` |
| `--idle-timeout` | `120s` |
| `--trust-project-header` | `false` |

## ClickHouse environment

Used only with `--storage=clickhouse`. Authentication is by client
certificate, not password, matching `clickhouse-migrate`.

| Env var | Default |
| --- | --- |
| `CLICKHOUSE_HOST` | required |
| `CLICKHOUSE_PORT` | `9440` |
| `CLICKHOUSE_USER` | `queryapi` |
| `CLICKHOUSE_DATABASE` | `o11y` |
| `CLICKHOUSE_TLS_CERT_FILE` | `/etc/clickhouse-client/certs/tls.crt` |
| `CLICKHOUSE_TLS_KEY_FILE` | `/etc/clickhouse-client/certs/tls.key` |
| `CLICKHOUSE_TLS_CA_FILE` | `/etc/clickhouse-client/certs/ca.crt` |

## Tenancy

Every request must resolve to a project or it is rejected with 401.

queryapi is reached only through Milo's aggregator/proxy chain. That chain
already authenticates the request, scopes it to a project (via its
`ProjectRouter`, which consumes the `/projects/{id}/control-plane` and
`/apis/{group}` path prefixes), and forwards the authenticated identity to us
as `X-Remote-*` headers. The project is therefore read from the delegated user
extras: `iam.miloapis.com/parent-name` (only when `iam.miloapis.com/parent-type`
is `Project`). A request carrying no such extra is unauthenticated.

For local development, `--trust-project-header` additionally enables the
`X-Project-Id` header. It is client-controlled and never set by the proxy chain,
so it is off by default. The `queryapi_project_source_total` metric reports
which source is in use.

Backends re-check the project themselves, so a handler bug cannot produce an
unscoped query.

## Local development

```sh
task queryapi:test
task queryapi:lint
go run ./cmd --trust-project-header   # the header source is off by default
# then: curl -H 'X-Project-Id: p' 'localhost:8080/v1/loki/api/v1/labels'
```
