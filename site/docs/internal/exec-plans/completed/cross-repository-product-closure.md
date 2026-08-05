# Cross-repository product closure: convergence record

Status: **closed** on 2026-08-04.

This document records the decisions that survived implementation review. The
original Phase 3–6 roadmap was intentionally retired: it coupled deployment,
configuration, protocol compatibility, UI copy, updaters, dependency policy,
and release governance into one completion claim. Those concerns have different
owners and failure modes and no longer block the current product loop.

There is no production or backwards-compatibility constraint for this closure.
Breaking changes are preferred when they remove duplicate authority or hidden
state.

## Resulting system boundary

Each supported path has:

1. one persisted user intent;
2. one runtime semantic authority;
3. one pinned update boundary; and
4. one deterministic local verification path.

Generated files may be duplicated because they are disposable projections.
Implementations that must remain behaviorally identical may not be duplicated.

## Completed decisions

### Deployment

- Photos publishes three explicit Hub Compose files: CPU, Vulkan, and CUDA.
- Linux Docker uses host networking. Vulkan maps `/dev/dri`; CUDA declares the
  Docker GPU device.
- The three files remain handwritten. A template generator would add another
  representation without reducing supported behavior.
- Host Broker is not part of the normal Docker or Desktop path.

### Release pins

- Photos owns independent `lumen.lock.json` and `assets.lock.json` files because
  Hub and Assets have independent release lifecycles.
- A lock records immutable upstream identity and hashes. Generated Desktop
  catalogs are checked against the lock locally.
- Normal CI does not contact GitHub. Remote tag, release, checksum, and
  provenance verification runs only during explicit reconciliation, release,
  or a dependency-boundary workflow.

### Hub configuration authority

- `lumen-schema` is the only implementation of preset and service semantics.
- Launcher, Docker, and `lumen-hub config render` translate their small input
  surfaces into the canonical renderer.
- Photos Desktop persists only setup intent (`preset`, `region`, and cache
  directory). Before every managed start, the exact pinned Hub binary renders
  its own Desktop configuration atomically.
- Photos does not own Hub model IDs, runtime names, service definitions,
  batching defaults, or YAML validation.
- Mounted Hub configuration and Docker configuration environment variables are
  mutually exclusive. There is no implicit merge mode.

### Protocol and discovery authority

- Discovery transports answer only where a node is. Descriptive version/runtime
  metadata may be carried for display, but task and protocol verdicts may not.
- SDK compatibility is established only by the data-plane capability RPC.
- Availability and compatibility are independent state dimensions.
- A node is `pending` until the capability stream proves it compatible or
  incompatible. Pending and incompatible nodes remain visible but are not
  routable.
- Endpoint or transport generation changes trigger a new verdict. Resolver
  metadata refreshes do not erase an existing in-band verdict.
- Host Broker sends complete address snapshots. It does not maintain a second
  incremental node-state machine.

### Support and publishing surface

- Host Broker remains source-buildable and experimental. SDK tags do not ship a
  cross-platform Host Broker binary matrix.
- Only platforms that can be exercised continuously should block a release.
  Build recipes for other profiles may remain without being advertised as
  supported products.
- Browser WASM tooling is opt-in and is not installed by the default repository
  setup.

## Intentionally cancelled work

The following items are not closure prerequisites and require their own plan
only after a concrete user need appears:

- a Site Docker wizard and generated full Hub YAML;
- CLI self-update;
- Desktop application auto-update;
- a cross-repository mega catalog for UI copy and documentation;
- a global Exact Term lint platform;
- a coordinated release train or aggregate release manifest;
- broad automatic Renovate merging;
- general plugin or extension-point infrastructure.

## Validation contract

The default repository gates are local and reproducible:

```text
task compose:test
task lumen:check
task assets:check
```

Remote dependency proof is explicit:

```text
task lumen:verify
task assets:verify
task dependencies:verify
```

A Hub upgrade is ordered: publish the Hub release first, then run
`task lumen:sync RELEASE=<tag>` in Photos and commit the resulting lock and
catalog together.

## Long-term constraints

- A semantic concept has one canonical writable representation.
- User intent and derived configuration cannot both be editable authorities.
- Network availability cannot decide an unrelated pull request.
- A compatibility branch must have an owner and deletion condition.
- A profile that is merely cross-compilable is not automatically supported.
- Do not create an extension point before a second real implementation exists.
