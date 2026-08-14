# Lumen Dynamic Discovery Convergence

Status: active. Investigation and target contracts were frozen on 2026-08-12.
The SDK convergence pipeline, local `v1.4.1` release tag, and Photos
backend/Monitor integration are implemented and verified; publishing the SDK
tag and real-LAN soak validation remain external gates.

Primary owners: `Lumen-SDK/pkg/discovery`, `Lumen-SDK/pkg/client`,
`server/internal/service/lumen_service.go`,
`server/internal/api/handler/capabilities_handler.go`, and
`web/src/features/monitor`.

This is a cross-repository correctness plan. Lumen SDK owns continuous node
discovery, connection convergence, capability negotiation, and diagnostic
snapshots. Lumilio Photos owns configuration mapping, de-sensitized API
projection, and truthful Monitor presentation. Lumen Hub participates in
contract fixtures and real-node validation, but its successful capability RPCs
are not the incident's failing boundary.

## Goal

Make Lumen discovery continuously convergent for a long-running Photos Server.
The Server and Hub may start, stop, warm up, or recover in any order. A Hub that
appears after Photos has been running for hours must become capability-routable
without restarting Photos, changing feature settings, or pressing a refresh
button.

The primary user journey is a required contract:

1. Start Photos Server with no Lumen Hub present and leave it running.
2. Start a valid LAN Lumen Hub later.
3. Enable `Image Semantic Analysis` and `Person Recognition` before or after the
   Hub appears.
4. Open Monitor and observe the validated node progressing from discovered to
   capability-pending to active.
5. Use the advertised tasks without restarting either process.

With the default timing configuration, a newly advertised Hub must become
active within one complete scan interval plus resolve, connect, capability, and
scheduling allowances. The default end-to-end acceptance bound is 50 seconds.
Normal LAN operation should converge much faster, but correctness may not rely
on the user reopening Monitor or manually refreshing it.

## Incident evidence and current diagnosis

The following facts were reproduced against Photos pinned to Lumen SDK
`v1.4.0` and a LAN Lumen Hub `v0.2.0`:

- The Hub advertises `_lumen._tcp.local` with valid descriptive TXT metadata
  and resolves to a reachable gRPC endpoint.
- A direct static-node client created through Photos' current
  `service.NewLumenService` obtains the `face` and `siglip` capabilities. Health,
  `GetCapabilities`, and `StreamCapabilities` all succeed.
- A newly started SDK client using mDNS also discovers the same Hub and obtains
  the same three task contracts.
- The already-running Photos process does not observe that Hub. Its public
  capability projection remains one discovered node, zero active nodes, and no
  available tasks.
- A temporary valid `_lumen._tcp.local` service added while that Photos process
  remained running was visible to the operating system's DNS-SD browser, but it
  was not added by the SDK after more than one configured scan cycle.
- Archived SDK logs show unrelated Bonjour services, including Spotify and SMB
  advertisements, accepted as Lumen nodes.
- `github.com/hashicorp/mdns v1.0.7`, used by the SDK, does not prove that every
  received response belongs to the requested service. The SDK then compounds
  this by letting `extractInstanceName` fall back to the first label when the
  `_lumen._tcp.local` suffix does not match.
- The Monitor's `discovered_node_count` is currently the pool's total node count,
  not proof of a validated Lumen advertisement or successful capability
  exchange.

These observations establish two defects and one remaining investigation
boundary:

1. **Foreign-service admission is confirmed.** Query results are not strictly
   correlated and the parser accepts suffix mismatches. The displayed
   discovered count can therefore be a false positive.
2. **Long-running non-convergence is confirmed.** The existing process failed
   to observe a valid hot-added service that the host DNS-SD stack observed.
3. **The exact liveness mechanism is not yet isolated.** It may be inside the
   mDNS query dependency, scan lifecycle, or delivery path. Phase 0 must make
   this failure deterministic before the production backend is replaced.

Capability negotiation is not the primary failure in this incident. The
existing client contract already retries a control-plane-first Hub whose gRPC
port is ready while `StreamCapabilities` is temporarily unavailable. That
behavior must be preserved and extended to the client-starts-first case.

## Root design problem

The current implementation treats a stream of loosely parsed DNS packets as if
it were a durable node-state authority. A polling goroutine can stop making
progress without an externally visible health state, individual query entries
can block downstream delivery, unsuccessful scans contribute the same absence
signal as successful empty scans, and aggregate pool counts conceal which stage
failed.

The target pipeline is instead a sequence of explicit, observable state
boundaries:

```text
bounded DNS-SD scan
        |
        v
strictly validated service snapshot
        |
        v
source-aware snapshot reconciliation
        |
        v
gRPC transport session
        |
        v
in-band capability verdict and task routing
        |
        v
Photos runtime projection -> authenticated Monitor
```

Each boundary owns one question:

- DNS-SD answers where a matching service is currently advertised.
- The resolver reconciles the latest successful observation with prior state.
- gRPC connectivity answers whether an endpoint is reachable now.
- The capability RPC alone answers protocol compatibility and supported tasks.
- Photos feature settings answer whether a user wants a capability enabled.

No boundary may infer the answer owned by another one.

## Non-goals

- Do not restart Photos Server when a Hub starts, stops, or changes readiness.
- Do not make ML capability switches start, reset, or rescan discovery.
- Do not make Monitor polling or its refresh button a correctness mechanism.
- Do not use mDNS TXT task names or versions as a compatibility or routing
  verdict. TXT metadata remains descriptive only.
- Do not make static nodes the workaround for broken LAN discovery. Static,
  Broker, and mDNS sources remain additive.
- Do not redesign Lumen inference payloads, model selection, indexing queues,
  or Hub model lifecycle.
- Do not expose raw DNS packets, arbitrary TXT values, internal errors, or LAN
  node details through the existing public `/api/v1/capabilities` route.
- Do not hand-edit generated OpenAPI or TypeScript schema files.

## Fixed convergence contracts

### Discovery admission

- A DNS-SD result is eligible only when its canonical PTR/SRV chain belongs to
  the configured service and domain, normally `_lumen._tcp.local`.
- Service and domain comparisons normalize case and trailing dots while
  preserving DNS label escaping. A suffix mismatch is rejected; it never falls
  back to a plausible-looking instance name.
- A valid observation has a non-empty instance identity, valid TCP port, and at
  least one usable address candidate. A matching service whose address cannot
  be resolved produces a typed resolution diagnostic, not a pool node.
- Unrelated multicast traffic is ignored and counted only as rejected input in
  bounded diagnostic counters. It cannot change node counts or expiry state.
- Identity comes from the correlated service instance plus configured
  deployment semantics. Hostname, IP, version, and runtime changes update the
  observation but do not silently manufacture a second identity.
- Discovery metadata never supplies tasks or a compatibility verdict.

Strict validation is required even if the mDNS dependency is replaced. A
library's filtering behavior is not the product's trust boundary.

### Scan lifecycle and liveness

- The mDNS backend performs an immediate scan when watching begins, followed by
  scans at the configured interval until its parent context is cancelled.
- Each scan has a hard context deadline and returns one immutable result:
  matched observations, rejected-entry count, start/completion timestamps, and
  a typed outcome.
- The production query backend must honor cancellation and release sockets and
  goroutines after every scan. Remove `github.com/hashicorp/mdns` from the
  production discovery path unless its behavior can satisfy the conformance
  harness; preserving it merely because it is already pinned is not a goal.
- Query collection is completed before publishing downstream changes. A slow
  event consumer cannot block DNS packet collection, query cleanup, or the next
  scheduled scan.
- Resolver delivery is latest-state convergent. Intermediate duplicate updates
  may be coalesced, but the newest complete snapshot may not be lost.
- A failed, timed-out, cancelled, or otherwise incomplete scan does not count as
  evidence that previously observed nodes disappeared.
- A node expires only after the configured number of consecutive successful,
  complete scans omit it. A later observation re-adds it on the same client
  lifecycle.
- If a watch loop exits unexpectedly while its parent context is live, the
  supervising resolver records the failure and restarts that backend with
  bounded backoff. One failed source does not stop static or Broker sources.
- Network loss, interface changes, machine sleep/wake, and multicast socket
  errors must recover through later scans without reconstructing
  `LumenClient`.
- `LumenClient.Start` may wait for initial readiness up to its configured
  timeout, but the timeout is not terminal. Discovery and capability
  negotiation continue until `Close` or parent-context cancellation.

### Multi-source reconciliation

- mDNS, Broker, and static resolvers publish source-owned snapshots. Expiring a
  node from one source cannot revoke an equivalent observation still owned by
  another source.
- Composite resolution is supervised per source. Failure or channel closure of
  one backend is diagnostic state, not a reason to close the aggregate stream.
- Reconciliation is idempotent: replaying the same snapshot produces no pool
  churn, capability refetch, or node-count growth.
- Endpoint generation changes produce a fresh transport and compatibility
  verdict. Descriptive metadata refreshes do not erase a valid in-band verdict.
- Duplicate addresses from multiple sources may share a transport only when
  identity and ownership rules prove they represent the same node. Address
  equality alone is insufficient identity.

### Transport and capability negotiation

- A newly resolved endpoint starts as transport-unavailable and
  compatibility-pending; it is not task-routable yet.
- When gRPC becomes ready, `StreamCapabilities` remains the preferred
  authoritative exchange, with the existing compatible fallback contract for
  older servers.
- `UNAVAILABLE`, deadline, EOF-before-any-capability, and transport replacement
  are retryable states with bounded backoff while the observation remains
  current. `UNIMPLEMENTED` and unsupported protocol major versions retain their
  explicit incompatibility semantics.
- A capability retry must not require a new discovery event. This preserves
  recovery when a Hub advertises and accepts gRPC before its services finish
  warming.
- A stale capability result from an older endpoint generation cannot overwrite
  the verdict for the current generation.
- Capability success atomically publishes the complete capability set used by
  `GetNodes`, `IsTaskAvailable`, task-contract lookup, and routing. Consumers
  must not observe a partially filled set.
- Loss and later recovery of the same Hub remove and restore task availability
  without recreating Photos' service object.

### State vocabulary

Discovery presence, transport availability, protocol compatibility, and user
enablement remain independent dimensions. Monitor may derive friendly copy,
but backend and SDK DTOs must preserve the dimensions.

| Dimension | Required states | Authority |
| --- | --- | --- |
| Discovery backend | `disabled`, `starting`, `healthy`, `degraded` | resolver supervisor |
| Observation | `resolved`, `expired`, `resolve_failed` | discovery source |
| Transport | `connecting`, `ready`, `unavailable` | gRPC pool |
| Compatibility | `pending`, `compatible`, `incompatible` | capability RPC |
| Feature | `enabled`, `disabled` | Photos runtime settings |

`active_node_count` means transport-ready and protocol-compatible.
`discovered_node_count` means currently resolved, strictly validated Lumen
observations. Neither count is inferred from a UI switch. Pending,
incompatible, and unavailable counts are reported separately so that
"discovered but not active" is diagnosable.

## SDK implementation boundary

The SDK change should remain backward-compatible for inference callers while
making resolver progress observable.

- Refactor `pkg/discovery/mdns_resolver.go` around an internal query interface
  that returns complete scan results. Production and fake query backends must
  pass the same conformance tests.
- Change mDNS conversion to return an explicit accepted/rejected result. Delete
  the permissive `extractInstanceName` fallback contract and tests that bless
  foreign suffixes.
- Reconcile successful snapshots inside the resolver and emit coalesced state
  changes only after a query completes.
- Add a read-only resolver diagnostic contract without adding methods to the
  existing `NodeResolver` interface. An optional status-provider interface
  keeps external resolvers and existing test doubles source-compatible.
- Extend `CompositeResolver` with per-source supervision and ownership-aware
  reconciliation. Its merged stream remains alive until the parent context is
  cancelled, even when one child exits unexpectedly.
- Plumb resolver status through `LumenClient` as an immutable runtime snapshot.
  It must include backend/source name, lifecycle state, last scan start,
  completion and success, consecutive failures, last typed error code, matched
  and rejected counts, and next scheduled scan. Do not expose raw packet data.
- Extend node snapshots with discovery source and last-observed time while
  retaining transport, compatibility, capability, version, and runtime fields.
- Keep `PoolStats` for compatibility, but stop treating it as the complete
  monitoring contract.
- Make `ResolveNow` request an expedited scan where the backend supports it;
  scheduled polling remains sufficient for correctness and requests are
  coalesced.

The first SDK release containing the fix must use a tagged module dependency in
Photos. A temporary `replace` is allowed only while validating both repositories
locally and must not remain in committed Photos configuration. If the changes
are additive, publish them as the next backward-compatible SDK tag; a breaking
`NodeResolver` change requires a separate major-version decision and is not
authorized by this plan.

## Photos backend and API contract

- `server/internal/service.LumenService` exposes one immutable runtime snapshot
  assembled by the SDK. The disabled implementation returns explicit disabled
  discovery state and empty node/task sets.
- `IsTaskAvailable` and inference routing continue to use compatible active
  capabilities. Feature settings do not mutate the Lumen client.
- The public `GET /api/v1/capabilities` route remains de-sensitized. It may add
  aggregate discovery health and state counts, but it must not return node
  identities, hostnames, addresses, arbitrary TXT metadata, or raw errors.
- Add `GET /api/v1/admin/lumen/runtime` as the authenticated administrator
  Monitor projection for per-backend and per-node diagnostics. It contains
  stable typed states, canonical service/task names, descriptive
  version/runtime, timestamps, source, and a bounded error code. Endpoint
  details are administrator-only; raw library errors remain in structured
  server logs.
- Monitor DTOs are OpenAPI-first. Update Go annotations and DTOs, run
  `task dto`, and consume generated TypeScript types without casts or hand edits.
- Structured log transitions include backend start/restart, scan completion or
  failure, node add/update/expiry, transport state, and capability verdict.
  Routine duplicate scans log at debug level. Logs use stable error codes and
  bounded fields rather than arbitrary DNS payloads.
- Metrics and logs must avoid unbounded labels derived from instance names,
  IPs, TXT records, or errors.

## Monitor contract

- The capabilities tab distinguishes discovery health, validated nodes,
  transport state, compatibility, and task availability.
- A valid newly observed Hub is shown as pending while transport or capability
  negotiation is in progress. It is not shown as an available task until the
  in-band verdict succeeds.
- A stalled/degraded backend is visibly different from a healthy scan that
  found zero nodes and from a node that is reachable but incompatible.
- The five-second query polling and Refresh action fetch the current snapshot
  only. The action is worded as status refresh and never restarts or rescans the
  SDK.
- `Image Semantic Analysis`, `Person Recognition`, `OCR Text Recognition`, and
  `BioCLIP Species Recognition` use the canonical capability labels. Protocol
  task identifiers remain unchanged inside technical detail rows.
- Enabled and available badges remain separate. Enabling or disabling a Photos
  feature updates only the enabled dimension and cannot hide resolver failure.
- Loading, empty, degraded, incompatible, and active states receive component
  tests. Node diagnostics are accessible without relying on color alone.

## Execution phases

### Phase 0 — Lock the incident as deterministic tests

- Add an injectable mDNS query seam and a scripted scan fixture to Lumen SDK.
- Add a client-starts-first test: initial successful empty scans, late valid
  Hub snapshot, gRPC connection, delayed capability readiness, and final task
  routing on the same client.
- Add a foreign-traffic test containing `_spotify`, `_smb`, UUID-named, and
  malformed entries mixed with one valid `_lumen._tcp.local` service. Only the
  valid service may enter the node snapshot.
- Add scan-outcome tests proving failed or timed-out queries do not increment
  expiry misses, while the configured number of successful omissions does.
- Add a blocked/slow-consumer test proving query completion, cleanup, and the
  next scan still progress.
- Add a child-watch termination test proving the composite supervisor restarts
  only that source and preserves other sources.
- Preserve `hub_startup_contract_test.go` and extend it so capability recovery
  is covered whether the endpoint exists before or after client startup.
- Record goroutine and socket cleanup assertions under cancellation and run the
  affected packages with the race detector.

Exit: the current false admission and hot-add non-convergence are reproducible
without relying on a physical LAN, and the tests fail against SDK `v1.4.0`.

### Phase 1 — Replace mDNS query and reconciliation semantics

- Implement strict service correlation and explicit accepted/rejected parsing.
- Replace or isolate the current HashiCorp query backend behind the conformance
  seam so every scan honors its deadline and cleans up resources.
- Publish only complete successful snapshots and apply failure-safe expiry
  rules.
- Coalesce downstream delivery and add an expedited scan trigger.
- Add backend lifecycle diagnostics and structured transition logging.

Exit: all Phase 0 discovery tests pass; repeated foreign multicast traffic
cannot affect the node set; a late scripted Hub converges within the configured
bound for thousands of scan cycles.

### Phase 2 — Supervise sources and preserve capability recovery

- Implement per-source ownership and restart supervision in the composite
  resolver.
- Preserve observations supplied by unaffected static or Broker sources while
  mDNS is degraded.
- Verify gRPC endpoint-generation and stale-capability guards during node
  restart, address change, and capability warmup.
- Publish one atomic SDK runtime snapshot spanning discovery status, nodes,
  transport, compatibility, and capabilities.

Exit: source failure, Hub restart, network loss/recovery, and delayed capability
startup converge on one client without goroutine growth or manual intervention.

### Phase 3 — Release SDK and integrate Photos

- Run the complete Lumen SDK CI contract and publish a compatible tagged
  release.
- Update `server/go.mod` and `server/go.sum` to the tag; remove any local
  `replace` directive.
- Map the SDK snapshot through `server/internal/service.LumenService` and its
  disabled implementation.
- Correct public aggregate count semantics and add the authenticated Monitor
  diagnostics DTO and handler.
- Add backend tests for no-Hub startup, late-node convergence projection,
  pending/incompatible/active distinctions, and setting-toggle independence.

Exit: a long-running Photos service observes a late SDK fixture, publishes its
capabilities, and never reports a foreign Bonjour entry as a Lumen node.

### Phase 4 — Make Monitor truthful

- Consume generated DTO types in the capabilities tab.
- Add backend-health and per-node state presentation with canonical capability
  labels and typed error copy.
- Keep auto-polling and status refresh side-effect free.
- Add component and browser coverage for zero-node healthy, backend degraded,
  capability pending, incompatible, active, expired, and recovered states.

Exit: the user can distinguish "no Hub advertised", "discovery degraded",
"Hub connecting", "capability pending", "incompatible", and "active" without
reading server logs.

### Phase 5 — Real LAN soak and documentation closure

- Run the required order permutations against a real LAN Hub: Server first,
  Hub first, Hub restart, Photos feature toggles before and after discovery, and
  network interface loss/recovery.
- Run an overnight long-lived Server soak with unrelated Bonjour traffic and
  repeated Hub add/remove cycles.
- Verify the managed Desktop static-node path and Broker discovery remain
  unchanged.
- Update `BACKEND.md` with the final snapshot and lifecycle authority, and
  update Monitor feature documentation through its canonical `doc.ts` flow.
- Complete this plan per [README.md](README.md): extract durable decisions to
  `.agents/decisions/`, then delete this file.

Exit: all automated and real-node gates below pass with no Server restart,
manual rescan, or feature-toggle side effect.

## Validation boundaries

### Required lifecycle scenarios

- Photos starts with no Hub; Hub advertises later.
- Hub advertises before Photos starts.
- Hub advertises before gRPC listens, and gRPC listens before capabilities are
  ready.
- Hub stops and restarts with the same identity and endpoint.
- Hub restarts with the same identity and a different address or port.
- One of several nodes expires while the others remain active.
- mDNS fails while a static or Broker source remains healthy.
- Wi-Fi or Ethernet disappears and returns; sleep/wake does not require a new
  client.
- A failed scan is followed by successful empty and successful non-empty scans.
- Foreign Bonjour traffic arrives before, during, and after valid Lumen replies.
- Photos capability switches change in every discovery/compatibility state.
- Monitor is never opened, is left open, and is refreshed repeatedly; runtime
  results are identical.

### SDK invariants

- Only strictly correlated service observations enter resolver state.
- Every scan either completes with one immutable result or records a typed
  failure by its deadline.
- Failed scans do not expire nodes.
- Successful snapshot replay is idempotent.
- A slow consumer cannot stop scans or cleanup.
- One resolver source cannot terminate the composite lifecycle.
- Capability results are generation-safe and atomically published.
- Closing the client cancels all scans, retries, watchers, streams, and
  supervisors without races or leaks.

### Repository gates

Run in Lumen SDK:

```text
go test -race ./pkg/discovery ./pkg/client
make ci
```

Run from the Lumilio Photos repository root:

```text
task ci:architecture
task dto
task server:test
task web:test
task web:test:browser
task ci:site
```

The real LAN acceptance run records timestamps for advertisement, validated
observation, transport readiness, capability success, and Monitor projection.
Under default configuration, the interval from advertisement to active
projection must not exceed 50 seconds. The overnight soak must show bounded
goroutine count, continued scan completions, zero admitted foreign services,
and successful recovery for every Hub add/remove cycle.

## Rollout and compatibility

- Land deterministic SDK tests before changing discovery production code.
- Release and pin the SDK before merging Photos DTO/UI behavior that depends on
  the new snapshot.
- Keep existing public capability fields during the rollout; correct their
  semantics and add fields rather than silently repurposing task identifiers.
- No database migration or runtime manifest fallback is required. Existing
  `[lumen]` scan, timeout, source, service, domain, and deployment values remain
  authoritative.
- A rollback returns Photos to the prior SDK tag and prior aggregate DTO. It
  must not require data repair.
- Do not ship a manual-restart instruction as mitigation. If the SDK release is
  not ready, accurately report discovery as degraded rather than presenting a
  false node count.

## Progress

- [x] Reproduce the real Hub through direct and fresh mDNS clients.
- [x] Reproduce failure to observe a hot-added service in the long-running
  Photos process.
- [x] Identify foreign-service admission and misleading aggregate counts.
- [x] Freeze convergence, ownership, API, and validation contracts in this
  plan.
- [x] Phase 0 — deterministic regression harness.
- [x] Phase 1 — strict mDNS scans and snapshot reconciliation.
- [x] Phase 2 — source supervision and capability recovery.
- [ ] Phase 3 — SDK release and Photos backend integration. SDK commit
  `5d105b2`, the local `v1.4.1` tag, the Photos pin, backend/API implementation,
  and local cross-module verification are complete; publishing the tag remains.
- [x] Phase 4 — Monitor diagnostics and UX.
- [ ] Phase 5 — LAN soak and documentation closure.

Local validation recorded on 2026-08-12:

- `go test -race ./pkg/discovery ./pkg/client` and `make ci` passed in Lumen SDK.
- `task dto`, `task server:test`, `task web:test`, `task ci:architecture`, and
  `task ci:site` passed in Photos using a temporary Go workspace for the sibling
  SDK module.
- `task web:test:browser` remains gated on publishing and pinning the SDK tag:
  the isolated E2E image intentionally builds from committed `server/go.mod`,
  which remains on `v1.4.0` until that release exists.
- The terminology checker now passes after the Collections map fallback key was
  renamed from the retired `library` literal to `repository`.

## Decision log

- 2026-08-12: Server-first and Hub-first startup orders are equally supported;
  restart is not a recovery contract.
- 2026-08-12: Feature enablement is independent of runtime discovery and
  availability.
- 2026-08-12: DNS-SD is an address-discovery source only; in-band capability RPCs
  remain the sole task and compatibility authority.
- 2026-08-12: mDNS queries publish validated complete snapshots, not unbounded
  packet-driven mutations.
- 2026-08-12: Failed scans preserve last-known observations; only successful
  omissions contribute to expiry.
- 2026-08-12: Resolver liveness is supervised and observable per source.
- 2026-08-12: The existing public capabilities route remains de-sensitized;
  detailed node diagnostics require administrator authentication.
- 2026-08-12: Monitor refresh observes state and never repairs it.
