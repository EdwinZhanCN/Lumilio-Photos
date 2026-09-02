# Decision: Lumen dynamic discovery convergence

Status: implemented — closed 2026-08-25. The SDK convergence pipeline, local
`v1.4.1` release, Photos backend/API and Monitor integration, and the real-LAN
soak are complete.

## Problem

Lumen discovery was not convergent for a long-running Photos Server. A Hub
that appeared after Photos had been running for hours stayed invisible or
incapability-routable until a restart or manual refresh; foreign Bonjour
services could be admitted and aggregate node counts could mislead. Discovery
was a single un-supervised query path rather than a reconciled, supervised
source lifecycle.

## Decision

Server-first and Hub-first startup orders are equally supported; restart is
not a recovery contract. Feature enablement is independent of runtime
discovery and availability. DNS-SD is an address-discovery source only;
in-band capability RPCs remain the sole task and compatibility authority.

mDNS queries publish validated complete snapshots, not unbounded packet-driven
mutations. Failed scans preserve last-known observations; only consecutive
successful omissions contribute to expiry. Every scan either completes with
one immutable result or records a typed failure by its deadline. The resolver
lifecycle is supervised and observable per source, and no single source can
terminate the composite lifecycle.

Photos consumes one immutable SDK runtime snapshot (`lumen-sdk v1.4.1`,
released and pinned with a refreshed checksum) and does not couple feature
switches, Monitor polling, or status refreshes to discovery progress. The
existing public capabilities route remains de-sensitized; detailed node
diagnostics require administrator authentication. Monitor refresh observes
state and never repairs it.

## Alternatives considered

- **Manual refresh or restart as mitigation** — rejected: correctness may not
  rely on the user reopening Monitor or restarting either process.
- **Packet-driven resolver mutations** — rejected: unbounded event streams
  cannot prove absence; validated complete snapshots with successful-omission
  expiry are the only sound basis for node expiry.
- **Monitor that repairs state** — rejected: presentation must be truthful
  about runtime state, not hide it behind UI-side repair.
- **Advertising mDNS as a capability authority** — rejected: DNS-SD carries no
  transport or task compatibility verdict; in-band capability RPCs own that.

## Close-out

- SDK `v1.4.1` released and pinned in Photos; contracts regenerated
  (`server/go.mod`, Desktop bump, checksum refresh).
- `GET /api/v1/capabilities` (aggregate, de-sensitized) and
  `GET /api/v1/admin/lumen/runtime` (authenticated diagnostics) are the
  Monitor boundary.
- Real-LAN soak, including Server-first/Hub-first orders, Hub restart with
  changed endpoint, interface loss/recovery, and overnight stability, was
  confirmed by the owner on 2026-08-25.
