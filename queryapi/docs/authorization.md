# Query API authorization

Before this design, every request that resolved to a project could read all of
that project's telemetry. Nothing asked whether the caller was allowed to.

This document records the decisions that closed that and the trade-offs each
one accepts. For the behaviour as shipped -- routes, permissions, flags,
deployment -- see [`../README.md`](../README.md).

## Milo authorizes; queryapi does not

queryapi takes authentication and authorization from `k8s.io/apiserver`'s
`DelegatingAuthenticationOptions` and `DelegatingAuthorizationOptions`. It
writes no review code and makes no tenancy decision of its own.

This works because tenancy already rides in the review. The delegating
authorizer copies the caller's `user.Info.Extra` into
`SubjectAccessReviewSpec.Extra`, so the `iam.miloapis.com/parent-type` and
`iam.miloapis.com/parent-name` extras the aggregator forwards arrive at Milo
intact, and Milo's IAM evaluates the permission against the project they name.
There is no project field in `ResourceAttributes` for queryapi to set, and none
to get wrong. The activity service relies on this in production today.

The RBAC this needs is `create` on `subjectaccessreviews.authorization.k8s.io`
plus the `extension-apiserver-authentication-reader` RoleBinding in
`kube-system` -- the same grants activity needs, for the same reasons.

## The framework serves the routes; it does not route them

`InstallAPIGroup` expects `rest.Storage` implementations behind resource
collections, and there are none here: `/loki/api/v1/label/{name}/values` is not
a subresource of anything. The routes stay on a plain `http.ServeMux` hung off
`NonGoRestfulMux`. Everything that has to be identical across API servers --
TLS, both delegating gates, health, metrics, and the discovery document -- comes
from the framework.

Adopting the runtime also fixed two breaks that predated it, either of which
would have stopped queries reaching production:

- kube-aggregator's availability controller probes `/apis/{group}/{version}`
  and marks the APIService unavailable on anything that is not a 2xx or 3xx.
  queryapi served nothing there, so the APIService registration was inert.
- kube-aggregator forwards the request path verbatim, so production sends
  `/apis/o11y.miloapis.com/v1alpha1/logs/...`. Only the shorter forms were
  registered, so the production path was a 404.

## Authorization is the framework's filter, not a parallel middleware

The first implementation ran its own authorization middleware between the
router and the handlers, because the stock `RequestInfoResolver` reads
`/apis/o11y.miloapis.com/v1alpha1/logs/loki/api/v1/tail` as `get logs` and would
have sent that to Milo. The concern was right; the remedy was not. Two review
paths in one process is the worse property: one more place to forget, and it
leaves the framework's own `WithAuthorization` filter in the chain with nothing
to do.

A custom `RequestInfoResolver` supplies the correct attributes to that filter
instead. It is safe only because it is fail-closed rather than best-effort:
under `/apis/o11y.miloapis.com` it never delegates to the stock resolver, and
`authz.Guard` denies any request under the group that the route table did not
produce. The README's [Why an unmapped path cannot be
served](../README.md#why-an-unmapped-path-cannot-be-served) has the full
argument.

## Three actions per resource, not one read permission

Label endpoints leak data that log bodies do not necessarily imply:
`/label/{name}/values` returns pod names, hostnames, and user and customer
identifiers. That is useful for autocomplete and sensitive on its own, and it is
a boundary that cannot be retrofitted once a single `logs.get` covers both.
`/series` is metadata too -- Loki returns matching stream label sets, no log
lines -- so it is named to parallel the label endpoints. `query` means "returns
log lines"; `get*` means "returns metadata about them".

All six permissions go in the `telemetry.miloapis.com-viewer` role today. The
split earns its keep the day someone wants a metadata-only role, and declaring
the actions now is free, whereas renaming later means migrating every IAM
policy.

## Semantic verbs, not method-derived verbs

Activity authorizes its query endpoints as `create` on `auditlogqueries`,
because in a server routing through `InstallAPIGroup` the verb is derived from
the HTTP method and its query resources are registered as `rest.Creater`. That
is a by-product of routing, not a considered convention. queryapi builds its own
`RequestInfo`, so it is free to name the action.

The resulting camelCase actions are a first for the platform: every
`ProtectedResource` shipped in `milo@v0.32.1` uses only the seven standard
verbs. Nothing forbids them -- the CRD puts no pattern on `spec.permissions`,
the `{service}/{resource}.{action}` grammar does not constrain the action, and
`ResourceAttributes.Verb` is a free-form string that Kubernetes itself uses for
`escalate` and `bind` -- but see [Open questions](#open-questions).

## Keep the `system:masters` bypass

`DelegatingAuthorizationOptions` sets `AlwaysAllowGroups: ["system:masters"]` by
default. queryapi keeps it, deliberately rather than by inheritance.

**It is required.** The availability probe authenticates as
`system:kube-aggregator` in `system:masters`. Without the bypass, that probe
would need a grant in Milo before the APIService could become available, which
makes the aggregator's health depend on IAM state. A failure there takes out
every query, not just the probe.

**It is a real hole.** Anything the front proxy presents as `system:masters`
reads any project's telemetry without a review, and the `ResourceAttributes`
never reach Milo. Two things stand between that and an arbitrary caller: the
front proxy's client certificate, which is what makes any `X-Remote-Group`
claim believable at all, and
[`config/queryapi/networkpolicy.yaml`](../../config/queryapi/networkpolicy.yaml),
which keeps traffic without such a certificate off the listener.

Activity accepts the same bargain, as does every aggregated API server built on
`RecommendedOptions`. Diverging is a change to make deliberately, with the
availability probe solved another way first.

## Alternatives considered

| Alternative | Why not |
| --- | --- |
| Send the review to the project's own Milo control plane | Shipped first, then replaced. Authentication and authorization are Milo's. It built a tenant boundary out of the endpoint address when the extras already carried tenancy into the review, and did so with queryapi-specific code -- an LRU of per-project authorizers, an address builder, its own flags -- in place of the framework's. It also assumed, unverified, that a project control plane serves `authorization.k8s.io/v1`. |
| Keep the parallel authorization middleware | Rejected once the resolver could be made provably fail-closed. Two review paths is the worse property. |
| Rewrite queryapi as a CRUD aggregated API server, like activity | Rejected. The service's value is Loki and Prometheus wire compatibility for Grafana datasources. Routing those paths through `InstallAPIGroup` means giving up that compatibility or fighting the framework on every route, and imports method-derived verbs. |
| Keep scraping `X-Remote-*` by hand and add only an authorization check | Rejected. It leaves identity-header integrity resting on the NetworkPolicy. The delegating authenticator closes that, and the only thing it needed that was not already there was a TLS listener. |
| Serve plain HTTP, with the NetworkPolicy as the trust boundary | Rejected. A NetworkPolicy says who may dial the port, not what they may claim once connected. Any workload that reaches the pod could assert any user, group and project. The client-CA check moves that from "we trust the network" to "we verified a certificate". |

## Testing

`handler_test.go` and `authorization_test.go` drive a real `httptest.Server` in
front of the assembled `GenericAPIServer`, so the handler chain, route table,
resolver and authorization chain under test are the ones a real process runs.
The single substitution is the front proxy's client certificate check, which a
plain-HTTP test server cannot satisfy; `deps_test.go` stands in the same
`headerrequest` parsing the delegating authenticator does after that check.

Covered: allow and deny per route against the permission each was reviewed for;
the review carrying the caller's name, groups and `iam.miloapis.com/parent-*`
extras; a failing review answering 500 rather than serving; an unmapped path
denied without a review; discovery served for the availability probe and denied
for everyone else; probes and `/metrics` answered with no credentials; an
authentication failure answering 401; and that no client-set header names the
project.

`routes_test.go` proves the resolver directly: every mapped route resolves to
its permission with no name, namespace or subresource, every unmapped path under
the group resolves to a non-resource request, and nothing under the group is
ever handed to the stock resolver.

## Open questions

1. **Does Milo's IAM authorizer round-trip an arbitrary SAR verb into a
   permission string, or does it hold a fixed verb-to-action map?** This gates
   the camelCase actions. The authorizer is not in the `milo` Go module, so it
   lives in a component neither repository contains, and no existing
   `ProtectedResource` uses a custom action. Resolve it with the Milo IAM
   owners, or empirically: register the `ProtectedResource`, bind the role, and
   send one `logs.getLabels` review to a real Milo. Failure is fail-closed --
   queries return 403 rather than leaking.
2. **Live tail.** `/loki/api/v1/tail` is declared in
   [`openapi.yaml`](../openapi.yaml) but unimplemented, and is denied today as
   an unmapped path. Per-request authorization does not cover a long-lived
   WebSocket, so it needs its own approach -- at minimum re-authorization on an
   interval -- decided before the endpoint ships.
3. **OpenAPI.** `OpenAPIConfig` and `OpenAPIV3Config` are nil, so `/openapi/v2`
   and `/openapi/v3` are not served and the aggregator's OpenAPI controllers log
   that they could not download a spec. Availability does not depend on it.
   Generating definitions would mean generating Go types for resources this
   service does not have; publishing [`openapi.yaml`](../openapi.yaml) there
   instead is the more likely answer.
