# Decision: Exercise Agent runtime through a keyless Ollama boundary

Status: implemented

## Problem

The browser Agent regression suite stopped at route-mocked frontend behavior.
It could not detect failures in authenticated SSE, Eino model/tool dialects,
SQLite bindings, confirmation checkpoints, run transitions, effect receipts,
or recovery after cancellation and provider errors. A repository gate could
not depend on public-provider credentials, paid model access, external network
availability, GPU inference, or nondeterministic model output.

The first real-stack scenarios exposed two lifecycle defects: omitted bindings
could be persisted as JSON `null` despite an array-shaped SQLite constraint,
and confirmation resume tried to create a second active run while the awaiting
run still occupied the unique user/thread index. The latter internal failure
was also incorrectly reported as Not Found.

## Decision

The isolated browser stack includes a deterministic `agent-model-fixture`
service implemented by `server/tools/fakeollama`. It exposes only the Ollama
protocol used by the pinned native Eino adapter, health, and bounded counters.
The E2E seed selects the real `ollama` provider with a fixed model and the
Compose-internal fixture URL; it supplies no API key. The fixture rejects
credential headers, unknown models, unknown scenarios, and unexpected tool
shapes, does not log request bodies, and derives every response from the
current request so retries and separate users do not share scenario state.

`@agent-runtime` owns real built-Web-to-Server coverage for plain streaming,
confirmation approval and rejection, stale duplicate confirmation,
cancellation and recovery, sanitized provider failure and recovery, and user
isolation. It does not intercept Agent, capabilities, or Settings traffic.
Tests prove mutations and isolation through public APIs and use fixture
counters only to establish provider-boundary facts. The slice remains separate
from the fast route-mocked `@agent-trust` suite and runs with one Playwright
worker against disposable seeded volumes. Every runtime test and repeat derives
its own authenticated user and repository identity; mutation scenarios also
create their own asset, album, and thread.

Missing thread bindings are normalized to JSON arrays at the Agent service
boundary. A resumed execution receives a distinct run in a non-active
`prepared_resume` state. After Eino accepts the checkpoint, one transaction
terminalizes the old awaiting run, activates the prepared run, and repoints
the thread. Build, checkpoint, or transition failures fail only the prepared
run and preserve the old confirmation for retry. The database still enforces
at most one active run per user/thread. Only missing, stale, and cross-user
resume identities map to 404; provider, checkpoint, and transaction failures
use the registered Agent failure boundary.

The current operational boundary is documented in
[`BACKEND.md`](../../docs/BACKEND.md#ml-lumen-and-llm) and the
[`lumilio-e2e-environment`](../skills/lumilio-e2e-environment/SKILL.md) skill.

## Alternatives considered

**Extend the route-mocked browser suite** — rejected because it cannot cross
the handler, Eino, checkpoint, SQLite, or effect boundaries that escaped.

**Add a deterministic provider or test branches to shipping Server code** —
rejected because test behavior would become a production configuration path
and would not prove the pinned native adapter's real wire contract.

**Use public provider keys in CI or developer E2E** — rejected because secrets,
entitlements, network state, cost, and provider uptime are not reproducible
repository invariants.

**Complete the old run before attempting checkpoint resume** — rejected
because a build or checkpoint failure would consume the user's still-valid
confirmation and make a safe retry impossible.

**Relax the one-active-run index or reuse the old run identity** — rejected
because concurrent executions would become possible or distinct executions
would lose their audit identity. A prepared state preserves both invariants.
