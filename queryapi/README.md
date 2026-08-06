# queryapi

Tenant-scoped log and metric query API over ClickHouse, serving `datumctl` and
the customer cloud portal. See [`openapi.yaml`](openapi.yaml) for the contract.

## Storage backends

| `--storage` | Behaviour |
| --- | --- |
| `fake` (default) | Deterministic synthetic logs, a pure function of the query's time range. Same range always returns the same rows. |
| `clickhouse` | Connects and reports reachability on `/readyz`; query endpoints return 501 until LogQL-to-SQL translation lands. |

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

Every request must resolve to a project or it is rejected with 401. The
project is read from forwarded identity, in order: the
`iam.miloapis.com/parent-name` user extra (only when
`iam.miloapis.com/parent-type` is `Project`), then `X-Project-Id`, then
`/projects/{id}/control-plane/` in the path. The `X-Project-Id` source is off
unless `--trust-project-header` is set, because nothing in the proxy chain
strips it and so it is client-controlled. The
`queryapi_project_source_total` metric reports which source is in use; once
confirmed in a real deployment, the unused sources should be deleted.

Backends re-check the project themselves, so a handler bug cannot produce an
unscoped query.

## Local development

```sh
task queryapi:test
task queryapi:lint
go run ./cmd --trust-project-header   # the header source is off by default
# then: curl -H 'X-Project-Id: p' 'localhost:8080/v1/loki/api/v1/label'
```
