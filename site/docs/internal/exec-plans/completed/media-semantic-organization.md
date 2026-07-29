# Media Semantic Organization — Completion Record

## Shipped goal

Lumilio Photos now has an owner-scoped, deterministic `events-v1` organization
surface whose membership atom is `media_item_id`. It preserves stable Event
identity across rebuilds, records user corrections separately from derived
membership, resolves redirects, reuses the existing Assets browse/search/viewer
stack, and works without ML/AI.

The final commit title is:

```text
feat(events): add deterministic media events and typed relations
```

## Final contracts

### Schema and domain

- Migration `000004_media_semantic_events` adds strict `events`,
  `event_media_items`, `event_constraints`, `event_redirects`,
  `event_dirty_ranges`, and `event_owner_state` tables with owner-aware foreign
  keys, active-membership uniqueness, checks, and indexes.
- `event.PolicyV1` centralizes the `events-v1` thresholds. Segmentation is
  deterministic over capture time, optional GPS, Stack folding, and explicit
  constraints; ML, OCR, faces, and LLM output never decide membership.
- Reconciliation retains stable IDs, deterministically assigns IDs across
  split/merge candidates, and writes direct redirects for retired Events.
- User title, cover, hidden state, include/exclude, must-link, and cannot-link
  intent is stored separately and survives automatic rebuilds.

### Rebuild and failure behavior

- Existing owners receive an idempotent initial backfill. New logical media and
  structural/user corrections create owner-scoped dirty ranges.
- River serializes rebuilds per owner, claims dirty ranges with a renewable
  lease, checks the owner revision before publish, recovers stale claims, and
  leaves the previous published Events intact after compute or publish failure.
- New dirty work arriving during a running job is retained and picked up by the
  recovery scheduler. Paused owners keep their dirty ranges.
- The SQLite and River paths use the catalog transaction; an enqueue failure
  rolls back the media mutation instead of publishing half of the state.

### API, authorization, and consumers

- Authenticated Event APIs provide stable cursor listing, redirect-aware detail,
  ordered assets, rename/cover/hide, rebuild controls, merge, split, move/add,
  remove, relations, and immutable sharing.
- Every Event, member, redirect, relation, rebuild, share, browse, and Agent
  lookup is scoped by the authenticated `owner_id`; cross-owner identifiers
  resolve as not found.
- Event browse is exposed through `AssetSetSourceEvent` and the generated
  OpenAPI `event_id` filter. The SPA composes the public `AssetBrowser` entry
  rather than maintaining a second gallery or search implementation.
- Event sharing resolves displayable assets and inserts the
  `asset_snapshot` share in one SQLite transaction. Existing shares never
  change after Event edits. The exact cap is 5,000 assets.
- Direct typed SQL exposes Event→Person/Album/Location, Person→Event, and
  Person→Person `co_occurs_with` relations. Counts normalize to distinct
  logical media. No graph projection or generic relation write API was added.
- Agent `lookup_events` and `filter_assets(event_id)` reuse the same owner-aware
  resolver and freeze exact ref snapshots. The Event ref cap is 10,000; an
  oversized Event returns `event_ref_too_large` without a partial ref.

### Product surface

- Routes `/collections/events`, `/collections/events/:eventId`, and
  `/collections/events/:eventId/:assetId` provide index, detail, and
  Event-scoped viewer navigation.
- Detail supports title/cover overrides, hide, merge, split, move, remove, and
  adding through the existing `PhotoPicker`.
- The feature-neutral `SelectedLogicalMedia` bulk-action contract carries
  logical media IDs and representative asset IDs while preserving complete
  Stack membership semantics.
- Feature documentation, generated OpenAPI/TypeScript contracts, and English
  and Chinese translations ship with the implementation.

## Durable recovery and authorization decisions

- Event identity and user intent are durable domain state; candidates,
  enrichment, dirty claims, and relation responses are rebuildable.
- A failed or stale worker cannot delete another worker's ranges or replace a
  newer owner revision.
- Redirects preserve old URLs, but authorization is rechecked against the
  target owner on every resolution.
- Public shares and Agent refs contain immutable authorized asset snapshots;
  neither a projection nor a `ResourceRef` is authorization evidence.
- Missing/deleted primary assets reduce displayable counts without deleting
  logical Event membership.

## Generators and local validation

Generated artifacts:

```bash
(cd server && sqlc generate)
make dto
(cd web && vp exec i18next-cli extract)
(cd web && vp exec i18next-cli status)
(cd web && vp node --input-type=module -e \
  'import { parseDocFile, renderMarkdown } from "@edwinzhancn/docts";
   import { writeFileSync } from "node:fs";
   const f = process.argv[1];
   writeFileSync(f.replace(/\.ts$/, ".md"), renderMarkdown(parseDocFile(f)));' \
  src/features/events/doc.ts)
```

Local validation on macOS ARM64:

```text
make server-test       PASS
make web-test          PASS — 67 files passed, 2 skipped; 280 tests passed, 6 skipped
make web-browser-test  PASS — 5 smoke tests, including the real-service Events path
make desktop-test      PASS — panel build and desktop Go/SQLite first/second launch
i18next-cli status     PASS — zh 100% (1656/1656)
Docker ARM64 build     PASS
git diff --check       PASS
```

No benchmark was run or added. Remote CI evidence, the final commit SHA, and the
PR URL are recorded in the PR body because they cannot be embedded in the commit
that they identify.
