# Authorizing the query API

**Status:** implemented. This document is kept for the rationale; see
[`../README.md`](../README.md) for the behaviour as shipped.

Before any of this, every request that resolved to a project could read *all*
of that project's telemetry. Nothing asked whether the caller was allowed to.
The sections below record how that was closed, what was tried first, and what
is still open.

## What is implemented

queryapi runs on `k8s.io/apiserver`'s `GenericAPIServer`, and takes three
things from it:

- **Serving.** TLS from `SecureServingOptions`, with a cert-manager
  certificate.
- **Authentication.** `DelegatingAuthenticationOptions`: the front proxy's
  client certificate is verified against the CA in the
  `extension-apiserver-authentication` ConfigMap before the `X-Remote-*`
  headers on that connection are believed, plus `TokenReview` for a caller
  presenting a bearer token directly.
- **Authorization.** `DelegatingAuthorizationOptions`: a `SubjectAccessReview`
  posted to Milo's apiserver. queryapi writes no review code.

What queryapi adds is the vocabulary and the routes, and one rule the framework
cannot know:

- a discovery document for `o11y.miloapis.com/v1alpha1`, so kube-aggregator
  will proxy to it at all;
- a custom `Config.RequestInfoResolver` that maps the Loki- and
  Prometheus-shaped routes onto `logs`/`metrics` x `query`/`getLabels`/`getSeries`;
- `authz.Guard`, which denies anything under this service's own group that the
  route table did not produce.

Etcd and admission are nil. queryapi persists nothing -- its data is in
ClickHouse -- and there are no writes to admit.

## Tenancy is in the review, not in the endpoint

The design first shipped here sent the review to the *project's own* Milo
control plane, addressed the way Milo addresses one
(`/apis/resourcemanager.miloapis.com/v1alpha1/projects/{id}/control-plane`, or
the in-cluster service under `--internal-service-discovery`), on the reasoning
that making the endpoint the tenant boundary left no project field to get
wrong. That is **replaced**. Authorization is delegated to Milo's apiserver,
exactly as the activity service delegates it.

The reason it works is that tenancy never needed a field of its own. The
delegating authorizer copies the caller's `user.Info.Extra` into
`SubjectAccessReviewSpec.Extra`, so `iam.miloapis.com/parent-type` and
`iam.miloapis.com/parent-name` -- the same two extras the aggregator forwards,
verified by the same client certificate check -- arrive at Milo intact, and
Milo's IAM evaluates the permission against the project they name. That is the
mechanism activity already relies on in production.

What went away with it: `ControlPlaneAuthorizer`, the per-project authorizer
LRU (the webhook authorizer's response cache is per instance, so per-project
authorizers meant per-project caches to hold), `DiscoveryConfig`,
`--project-kubeconfig`, `--internal-service-discovery`, and
`--authorization-project-cache-size`, which only existed to bound that LRU. The
review cache TTLs are now the framework's own
`--authorization-webhook-cache-{authorized,unauthorized}-ttl`, with the same
10s defaults. The RBAC is unchanged: `create` on
`subjectaccessreviews.authorization.k8s.io` plus the
`extension-apiserver-authentication-reader` RoleBinding in `kube-system`, which
is what activity needs and for the same reasons.

This also moots the question of whether a project control plane serves
`authorization.k8s.io/v1` at all. It no longer has to.

## Why the framework serves the routes, when it once looked like it could not

An earlier version of this document rejected adopting the apiserver framework
outright, on the grounds that "the service's value is Loki and Prometheus wire
compatibility for Grafana datasources, and those paths are not Kubernetes
resource paths". That was true of one thing and untrue of three others, and the
distinction matters enough to write down.

**True:** the framework cannot *route* these paths. `InstallAPIGroup` wants
`rest.Storage` implementations behind resource collections, and there are none
here -- `/loki/api/v1/label/{name}/values` is not a subresource of anything. The
routes stay on a plain `http.ServeMux`, hung off `NonGoRestfulMux`.

**Untrue:** that this stopped the framework from *serving*, *authenticating* or
*authorizing* them.

- Serving: `Handler.NonGoRestfulMux` exists precisely so a server can attach
  handlers the go-restful container knows nothing about.
- Authorizing: `Config.Authorization.Authorizer` is a plain
  `authorizer.Authorizer`, and `Config.RequestInfoResolver` is a plain
  `RequestInfoResolver`. Nothing requires the attributes to have come from
  parsing a Kubernetes resource path; a route table produces better ones.
- Discovery: `/apis/{group}/{version}` is a document, not a consequence of
  having resources. Serving an empty one is legitimate and honest.

**And the cost of not adopting it was not zero, which is the part that was
missed.** kube-aggregator's availability controller
(`pkg/controllers/status/remote/remote_available_controller.go`) probes
`GET /apis/{group}/{version}` on the backend, sending `X-Remote-User:
system:kube-aggregator` and `X-Remote-Group: system:masters`, and requires a
2xx or 3xx. Anything else marks the APIService `NotAvailable`, and the
aggregator will not proxy a single request to a backend it has marked
unavailable. queryapi served nothing at that path, so the APIService
registration was inert: not one query would have arrived in production.

The same investigation turned up a second, quieter break.
`handler_proxy.go` forwards the request path verbatim, so production sends
`/apis/o11y.miloapis.com/v1alpha1/logs/loki/api/v1/query_range`. The mux
registered only `/v1alpha1/logs/...` and rewrote only Grafana's bare
`/loki/api/v1/...` form, so the production path was a 404. Both are fixed by
the same change, and all three forms now normalise onto the aggregator's own
before anything else looks at the request.

## Why authorization is the framework's filter, not a parallel middleware

The first implementation ran its own `authz.Middleware` between the router and
the handlers, because the stock `RequestInfoResolver` would have read
`/apis/o11y.miloapis.com/v1alpha1/logs/loki/api/v1/tail` as `get logs` and sent
*that* to Milo. The concern was right; the remedy was the wrong one. Two review
paths in one process is a worse property than one, and it left the framework's
own `WithAuthorization` filter sitting in the chain with nothing useful to do.

A custom `RequestInfoResolver` closes it properly, but only because it is
written to be fail-closed rather than best-effort:

- Under `/apis/o11y.miloapis.com` **nothing is delegated to the stock
  resolver.** A path either matches the route table, or it does not.
- A match is described as a resource request for that route's permission --
  and matched by `http.ServeMux.Handler` on the tenant mux itself, so it is the
  same matcher, on the same path string, that will pick the handler. There is
  no second parser to disagree with the first. A wrong method, or a path the
  mux would redirect, reports no pattern and is therefore not a match.
- A non-match is described as a **non-resource** request. It carries no
  resource and no verb that any policy could be written against, and -- this is
  the load-bearing part -- the tenant mux serves nothing at such a path, so
  even an allow reaches a 404 rather than a handler.
- `authz.Guard` then denies it before the review happens at all. Under this
  service's group only two things reach Milo: a discovery document, and a
  resource request naming a permission in the closed set in
  `internal/authz/permissions.go`. An unmapped path is never asked about as a
  non-resource URL.

Outside the group the guard is a pass-through, so `/healthz`, `/metrics`,
`/apis` and `/version` are decided by the framework's own chain, unchanged.

`--authorization-always-allow-paths` deserves its own note, because it is the
one flag that removes a gate. It cannot open a query route:
`path.NewAuthorizer` abstains on every resource request, and a query route is a
resource request by the time it is authorized. `Options.Validate` refuses a
value naming `/apis/o11y.miloapis.com` anyway, on the theory that an operator
who wrote it believed it would work and should be told at startup that it does
not.

## The `system:masters` bypass

`DelegatingAuthorizationOptions` sets `AlwaysAllowGroups: ["system:masters"]`
by default, and queryapi keeps it. Recording it here rather than letting it be
inherited silently:

**It is required.** The availability probe above authenticates as
`system:kube-aggregator` in `system:masters`. Without the bypass the probe
would have to be granted in Milo before the APIService could become available,
which makes the aggregator's health depend on IAM state -- and a failure there
takes out every query, not just the probe.

**It is a real hole.** Anything the front proxy presents to queryapi as
`system:masters` reads any project's telemetry without a review, and the
`ResourceAttributes` never reach Milo. Two things stand between that and an
arbitrary caller: the front proxy's client certificate, which is what makes any
`X-Remote-Group` claim believable at all, and
[`config/queryapi/networkpolicy.yaml`](../../config/queryapi/networkpolicy.yaml),
which keeps traffic that has no such certificate off the listener.

**Activity accepts the same bargain**, as does every aggregated apiserver built
on `RecommendedOptions`. Diverging from it would be a change to make
deliberately, with the availability probe solved another way first, not by
clearing the field and discovering the consequence in production.

The ClickHouse row policy is not bypassed by any of this: it keys on the
`telemetry_project_id` setting, which is bound from the resolved project rather
than from the review. A `system:masters` caller still has to be acting inside a
project to read anything, because `miloauth.Middleware` refuses to run a
handler otherwise.

## Permission vocabulary

Milo permissions are `{service}/{resource}.{action}`, and o11y already shipped
`ProtectedResource` and `Role` CRDs under
[`config/operator/iam/`](../../config/operator/iam/) using
`telemetry.miloapis.com/exportpolicies.*`. The query surface adds
`ProtectedResource`s for `logs` and `metrics` under the `o11y.miloapis.com`
service:

| Route (under `/apis/o11y.miloapis.com/v1alpha1`) | Permission |
| --- | --- |
| `/logs/loki/api/v1/query`, `.../query_range` | `o11y.miloapis.com/logs.query` |
| `/logs/loki/api/v1/labels`, `.../label/{name}/values` | `o11y.miloapis.com/logs.getLabels` |
| `/logs/loki/api/v1/series` | `o11y.miloapis.com/logs.getSeries` |
| `/metrics/api/v1/query`, `.../query_range` | `o11y.miloapis.com/metrics.query` |
| `/metrics/api/v1/labels`, `.../label/{name}/values` | `o11y.miloapis.com/metrics.getLabels` |
| `/metrics/api/v1/series` | `o11y.miloapis.com/metrics.getSeries` |

The metrics rows are wired even though those handlers return 501 today. Gating
a stub costs nothing and avoids the endpoint shipping unguarded later.

Three actions rather than a flat read permission, because label endpoints leak
data that log bodies do not necessarily imply: `/label/{name}/values` returns
pod names, hostnames, user and customer identifiers. That is useful for
autocomplete and sensitive on its own, and it is a boundary that cannot be
retrofitted once a single `logs.get` covers both. `/series` is metadata too --
Loki returns matching stream label sets, no log lines -- so it is named to
parallel the label endpoints. `query` means "returns log lines"; `get*` means
"returns metadata about them".

All six go in `telemetry-viewer`. The split only earns its keep the day someone
wants a metadata-only role, but declaring the actions now is free, whereas
renaming later means migrating every IAM policy.

These are camelCase actions, which would be a first for the platform: every
`ProtectedResource` shipped in `milo@v0.32.1/config/protected-resources/` uses
only the seven standard verbs. It is structurally allowed -- the CRD puts no
pattern on `spec.permissions` items, the permission grammar does not constrain
`{action}`, and `ResourceAttributes.Verb` is a free-form string that Kubernetes
itself uses for non-CRUD verbs like `escalate` and `bind` -- but see the open
questions.

Activity's verb choice is deliberately not copied. Its query endpoints
authorize as `create` on `auditlogqueries` because in an aggregated apiserver
serving real resources the verb is derived from the HTTP method and its query
resources are registered as `rest.Creater`. That is a by-product of routing
through `InstallAPIGroup`, not a considered convention. Because queryapi builds
its own `RequestInfo`, it is free to choose semantic verbs.

## Keep the existing defence in depth

The SAR is an additional gate above the ClickHouse row policy, not a
replacement:

```sql
CREATE ROW POLICY IF NOT EXISTS queryapi_project_isolation
ON logs FOR SELECT
USING ProjectId = getSetting('telemetry_project_id')
```

Critically, the project Milo evaluated the review against and the project bound
to `telemetry_project_id` come from one `miloauth.Resolve` on one verified
`user.Info`. If those two could ever diverge, a caller authorized for one
project would read another.

`config/queryapi/networkpolicy.yaml` is deployed as defence in depth rather
than as the trust boundary. It says who may dial the port; it says nothing
about what they may claim once connected. The client-CA check is what moved
that from "we trust the network" to "we verified a certificate", and is why TLS
came into scope here at all.

## Testing

`handler_test.go` and `authorization_test.go` drive a real `httptest.Server` in
front of the assembled `GenericAPIServer` -- the same handler chain, route
table, request info resolver and authorization chain a real process runs. The
one thing stood in for is the front proxy's client certificate check, which a
plain-HTTP test server cannot satisfy; `deps_test.go` substitutes the same
`headerrequest` parsing the delegating authenticator does after that check, and
`options.go` is the only place the check itself is configured.

Covered: allow and deny per route with the permission each was reviewed for;
the review carrying the caller's name, groups and `iam.miloapis.com/parent-*`
extras; a review that errors answering 500 rather than serving; an unmapped
path denied without a review; the discovery documents served for the
availability probe and denied for everyone else; probes and `/metrics` answered
with no credentials; an authentication failure answering 401; and that no
client-set header names the project.

`routes_test.go` proves the resolver directly, which the HTTP layer cannot: it
asserts that every mapped route resolves to its permission with no name,
namespace or subresource, that every unmapped path under the group resolves to
a non-resource request, and that nothing under the group is ever handed to the
stock resolver.

## Open questions

1. **Does Milo's IAM authorizer round-trip an arbitrary SAR verb into a
   permission string, or does it hold a fixed verb-to-action map?** This blocks
   the camelCase actions above. The authorizer is not in the `milo` Go module,
   so it lives in a component neither repository contains. Given that no
   existing `ProtectedResource` uses a custom action, an unknown verb mapping
   to nothing is plausible. Resolve with the Milo IAM owners, or empirically:
   register the `ProtectedResource`, bind the role, and SAR a `logs.getLabels`
   against a real Milo before relying on it.
2. ~~**Does the project control plane serve `authorization.k8s.io/v1` SARs at
   all?**~~ No longer applicable: reviews go to Milo's apiserver, which serves
   them for the activity service today.
3. **Live tail.** `/loki/api/v1/tail` is declared in
   [`openapi.yaml`](../openapi.yaml) but unimplemented, and is currently denied
   as an unmapped path -- which is the right answer while it does not exist.
   Per-request authorization does not cover a long-lived WebSocket, so it needs
   its own approach -- at minimum re-authorization on an interval -- decided
   before the endpoint ships.
4. **OpenAPI.** `OpenAPIConfig` and `OpenAPIV3Config` are nil, so `/openapi/v2`
   and `/openapi/v3` are not served and the aggregator's OpenAPI controllers
   will log that they could not download a spec. Availability does not depend
   on it. Generating definitions would mean generating Go types for resources
   this service does not have; publishing [`openapi.yaml`](../openapi.yaml)
   there instead is the more likely answer, and is unresolved.

## Alternatives considered

**Keep the per-project control-plane authorizer.** Superseded, and by a
decision rather than a discovery: authentication and authorization are Milo's.
The per-project design was not wrong so much as unnecessary -- it built a
tenant boundary out of the endpoint address when the extras already carried
tenancy into the review, and it did so with queryapi-specific code (an LRU of
authorizers, an address builder, its own config and flags) in place of the
framework's. It also rested on an unverified assumption about what a project
control plane serves.

**Keep the parallel authorization middleware.** Rejected once the resolver
could be made provably fail-closed. Two review paths is the worse property: it
is one more place to forget, and it leaves the framework's own filter in the
chain doing nothing.

**Rewrite queryapi as a CRUD aggregated API server, exactly like activity.**
Rejected, and this part of the original reasoning stands. The service's value
is Loki and Prometheus wire compatibility for Grafana datasources. Routing
those paths through `InstallAPIGroup` would mean either giving up that
compatibility or fighting the framework on every route, and would also import
activity's method-derived verbs. Taking the runtime without the resource
routing gets the discovery document, the identity chain and the review path
without either cost.

**Keep scraping `X-Remote-*` by hand and add only an authorization check.**
Rejected. It leaves identity-header integrity resting on the NetworkPolicy,
which is the gap `networkpolicy.yaml` itself flagged. The delegating
authenticator closes it, and the only thing it needed that was not already
there was a TLS listener.

**Serve plain HTTP and keep the NetworkPolicy as the trust boundary.**
Rejected. A NetworkPolicy says who may dial the port; it says nothing about
what they may claim once connected. Any workload that can reach the pod --
through a policy widened by mistake, a shared namespace, a sidecar -- could
assert any user, any group and any project, and queryapi would believe all of
it. The client-CA check moves that from "we trust the network" to "we verified
a certificate". The policy stays, now as defence in depth: it keeps
unauthenticated traffic off the listener rather than letting it fail at the
handshake, and it is what limits who can assert `system:masters`.
