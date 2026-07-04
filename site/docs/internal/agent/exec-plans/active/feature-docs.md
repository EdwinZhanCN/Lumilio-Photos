# Feature Documentation Review

## Goal

Bring the feature-driven frontend code, feature architecture docs, and Chinese
user manual back into alignment.

The immediate output is a reviewed set of Chinese feature docs under
`site/docs/zh-cn/user-manual/features/` plus any needed `doc.ts` architecture
docs under `web/src/features/<feature>/`. The work should make user-visible
features discoverable, remove stale behavior descriptions, and keep internal
architecture prose tied to real code symbols through `docts`.

## Scope

In scope:

- Chinese user-facing feature documentation only.
- Frontend feature directories under `web/src/features/`.
- Route coverage from `web/src/routes/routes.tsx`.
- Feature architecture docs using the `doc.ts` convention.
- VitePress sidebar updates when a new Chinese feature page becomes public.
- Contract/type red flags that directly affect whether docs can describe the
  feature accurately.

Out of scope:

- Installation, deployment, or environment setup docs.
- English docs parity.
- Screenshots, unless a later pass explicitly supplies images.
- Product redesign or feature implementation unrelated to documentation drift.
- Hand-editing generated files, including `doc.md`, OpenAPI schema, or locale
  JSON structure.

## Ground Rules

- Read the relevant feature code before editing docs. Do not infer behavior
  from old prose when code disagrees.
- Public docs are Chinese-first and user-facing. Avoid internal API names unless
  they explain a user-visible concept.
- `doc.ts` is the source for feature architecture docs; `doc.md` is generated
  and must not be manually edited.
- Every `{@link Symbol}` in `doc.ts` must have a matching `import type`.
- If a generated API type is missing data and frontend code casts around it,
  treat the contract as suspect; do not document guessed behavior as fact.
- Preserve repository scoping rules: browse scope is for list pages, working
  repository is for upload, entity mutations use entity identity, and Manage
  owns maintenance actions.

## Initial Findings

- `web/src/features` currently has feature directories for settings, home,
  studio, monitor, lumilio, auth, updates, users, people, portfolio, manage,
  collections, assets, upload, and share.
- Only `assets`, `collections`, `people`, and `share` currently have feature
  `doc.ts` architecture docs.
- Chinese feature docs currently cover home, assets, collections, albums,
  utilities, studio, manage, settings, and monitor.
- User-visible routes exist for public share links (`/s/:token`), shared-link
  management, Liked, folders, tags, trash, people detail, Lumilio chat, and
  several collection detail pages.
- `utilities.md` still describes utilities as duplicates, trash, and smart
  classifiers only, while the current utility shortcut source also includes
  Liked, Tags, and Shared Links.
- Sharing has an internal feature doc, owner-facing management page, public
  viewer, gallery bulk action, full-screen single-asset share, and album/person
  collection-level share buttons, but no Chinese user manual page yet.

## Step 1 Coverage Matrix

Status labels:

- `covered`: current docs exist and no obvious route-level gap was found in the
  first pass.
- `stale`: docs exist but miss current user-visible behavior.
- `missing-user-doc`: a user-visible route or feature lacks Chinese user docs.
- `missing-doc-ts`: a feature has enough architecture surface to need a
  `web/src/features/<feature>/doc.ts` decision.
- `internal-only`: no public feature page is needed in this review.

### Route Coverage

| Route / area | Code owner | Chinese user doc | Sidebar | `doc.ts` | Status | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `/` | `home` | `home.md` | yes | no | `covered`, `missing-doc-ts` | Dashboard docs exist; feature architecture doc should be considered because the page composes multiple scoped data hooks. |
| `/assets`, `/assets/:assetId` | `assets` | `assets.md` | yes | yes | `stale` | Docs cover core browsing/filtering, but miss current sharing entry points and still contain a placeholder for Lumilio Agent docs. |
| `/collections` | `collections` | `collections.md` | yes | yes | `stale` | Current hub includes folders, tags, liked, shared links, and utility shortcuts beyond the older prose. |
| `/collections/albums`, `/collections/:albumId` | `collections` | `albums.md` | yes | yes | `stale` | Album docs miss whole-album share and selected-item share behavior. |
| `/collections/map`, `/collections/places/:tripId` | `collections` | `collections.md` | yes | yes | `covered` | First pass found map/trip coverage at the user level; keep in drift audit. |
| `/collections/people`, `/people/:personId` | `collections`, `people` | `collections.md` | indirect | yes (`people`) | `stale` | People list/detail are only lightly documented; correction tools, hidden people, face moves/merges, and share behavior need user docs. |
| `/collections/utilities` | `collections` | `utilities.md` | yes | yes | `stale` | `utilities.md` says the hub has three tools, but `useUtilityShortcuts` now includes Liked, Duplicates, Trash, Tags, Shared Links, and classifier albums. |
| `/collections/utilities/duplicates` | `collections` | `utilities.md` | yes | yes | `covered` | Duplicate docs exist; later pass should verify current delete/merge semantics against code before editing nearby sections. |
| `/collections/utilities/:classifierSlug` | `collections` | `utilities.md` | yes | yes | `covered` | Smart classifier albums are covered at a high level. |
| `/collections/trash` | `assets` via collections | `utilities.md` | yes | yes (`assets`) | `covered` | Trash docs exist; wording should be checked for permanence/restore semantics during the utilities rewrite. |
| `/collections/liked` | `collections` | none dedicated | no | yes (`collections`) | `missing-user-doc` | Liked is a scoped asset gallery with a special "remove from Liked" bulk action. |
| `/collections/folders` | `collections` | none dedicated | no | yes | `missing-user-doc` | Folders are derived from repository paths, browse-scope aware, and not editable albums. |
| `/collections/tags` | `collections` | assets tag sections only | no | yes | `missing-user-doc` | Tags page is a tag-summary browse entry with manual/AI source filters; it differs from asset filter tag picking. |
| `/collections/shared-links` | `share` | none | no | yes (`share`) | `missing-user-doc` | Owner-facing share management supports revoke, extend, and delete but cannot re-copy old raw tokens. |
| `/s/:token`, `/s/:token/:assetId` | `share` | none | no | yes | `missing-user-doc` | Public recipient viewer is outside auth/setup gates and needs a user-facing explanation. |
| `/manage`, `/upload-photos` redirect | `manage`, `upload` | `manage.md` | yes | no | `covered`, `missing-doc-ts` | User docs cover upload and repository management; `manage` and `upload` need architecture-doc decisions. |
| `/studio` | `studio` | `studio.md` | yes | no | `covered`, `missing-doc-ts` | User docs exist; architecture doc should capture home/editor/worker/tool boundaries. |
| `/settings` | `settings`, `users` | `settings.md` | yes | no | `covered`, `missing-doc-ts` | User docs cover account, appearance, server, AI, cloud, users; architecture doc should capture settings drafts and repository scope hooks. |
| `/server-monitor` | `monitor` | `monitor.md` | yes | no | `covered`, `missing-doc-ts` | User docs exist; architecture doc should capture queue/ML/capability polling seams. |
| `/lumilio` | `lumilio` | none | no | no | `missing-user-doc`, `missing-doc-ts` | User-visible chat/board route exists; assets doc only references it with TODO placeholders. |
| `/login`, `/register`, `/mfa`, `/change-password`, `/bootstrap` | `auth` | none dedicated | no | no | `internal-only`, `missing-doc-ts` | Treat as onboarding/account-flow docs deferred for this feature-only pass unless the scope expands. Architecture doc is still a candidate. |
| `/updates` | `updates` | none | no | no | `internal-only` | Route is commented out. |
| `/portfolio` | `portfolio` | none | no | no | `internal-only` | Route is commented out. |

### Feature Directory Coverage

| Feature dir | User-doc status | Architecture-doc status | Decision |
| --- | --- | --- | --- |
| `assets` | `stale` | `covered` | Update user docs for share/Lumilio references; no new `doc.ts` needed in this pass. |
| `collections` | `stale` | `covered` | Update collections/utilities docs for current route set. |
| `people` | `stale` via `collections.md` | `covered` | Add or expand user docs for person detail corrections and sharing. |
| `share` | `missing-user-doc` | `covered` | Add Chinese sharing docs and sidebar entry. |
| `lumilio` | `missing-user-doc` | `missing-doc-ts` | Needs both public docs and architecture doc if `/lumilio` remains a first-class route. |
| `settings` | `covered` | `missing-doc-ts` | Keep user docs; add architecture doc decision. |
| `manage` | `covered` | `missing-doc-ts` | Keep user docs; add architecture doc decision. |
| `upload` | covered inside `manage.md` | `missing-doc-ts` | No separate user page unless Manage grows too large; architecture doc candidate. |
| `studio` | `covered` | `missing-doc-ts` | User docs exist; architecture doc candidate. |
| `monitor` | `covered` | `missing-doc-ts` | User docs exist; architecture doc candidate. |
| `home` | `covered` | `missing-doc-ts` | User docs exist; architecture doc candidate. |
| `auth` | deferred | `missing-doc-ts` | Public onboarding docs out of current feature-doc scope; architecture doc candidate. |
| `users` | covered inside `settings.md` | `internal-only` | No separate user page or `doc.ts` unless users grows outside settings. |
| `updates` | `internal-only` | `internal-only` | Route disabled. |
| `portfolio` | `internal-only` | `internal-only` | Route disabled. |

### Step 1 Priority Order

1. Rewrite `utilities.md` around the current shortcut set.
2. Add a sharing user doc and link it from the sidebar.
3. Add or integrate user docs for Liked, Folders, Tags, and Shared Links.
4. Update `collections.md`, `albums.md`, and people-related prose for sharing
   and correction flows.
5. Decide whether Lumilio gets its own user page in this pass; if yes, remove
   the placeholder from `assets.md` and link to it.
6. Defer the broader `doc.ts` expansion until user-facing drift is corrected,
   except for `share`/`lumilio` details needed to avoid ambiguity.

## Progress

- Step 1 complete: coverage matrix added above.
- Step 2 batch 1 complete:
  - Rewrote `site/docs/zh-cn/user-manual/features/utilities.md` around the
    current shortcut set: Liked, Duplicates, Trash, Tags, Folders, Shared
    Links, and smart classifier views.
  - Added `site/docs/zh-cn/user-manual/features/sharing.md` for owner-facing
    share creation/management and recipient-facing public share viewing.
  - Added the Sharing page to `site/docs/.vitepress/sidebar/zh-cn.ts`.
  - Updated `collections.md`, `assets.md`, and `albums.md` where they were
    missing current sharing, folders, tags, and utility behavior.
- Validation: `pnpm vitepress build` completed with exit code 0 after network
  permission allowed pnpm to fetch `@pnpm/exe`. The build printed existing
  Lucide Vue SSR errors from `assets.md` icon component syntax (`<Search />`,
  `<Plus />` style), but still finished successfully. Treat that as a separate
  docs-site cleanup item if strict stderr-free builds are required.
- Step 2 batch 2 complete:
  - Added `site/docs/zh-cn/user-manual/features/people.md` for people list,
    detail, naming, hidden people, face correction, merge, sharing, and rebuild
    behavior.
  - Added `site/docs/zh-cn/user-manual/features/lumilio.md` for the Agent
    entry points, modes, `@` mentions, context chips, response blocks, board
    pins, view switching, sizing, and new conversations.
  - Updated the Chinese sidebar with People and Lumilio Agent pages.
  - Updated `collections.md` to link to the new People page and replaced the
    old people summary with current correction/share behavior.
  - Replaced the Lumilio Agent TODO in `assets.md` with a real link.
  - Validation: `pnpm vitepress build` completed with exit code 0. It still
    prints the same pre-existing Lucide Vue SSR errors from `assets.md`.
- Step 3 batch 1 complete:
  - Added `web/src/features/lumilio/doc.ts` and generated
    `web/src/features/lumilio/doc.md` with `docts`.
  - The architecture doc covers the `/lumilio` route, chat dock, context store,
    SSE stream, typed blocks, mentions, slash modes, inline widgets, board pins,
    widget registry, and board durability decisions.
  - Validation: `make web-test` passed.
- Step 3 batch 2 complete:
  - Added `web/src/features/settings/doc.ts` and generated
    `web/src/features/settings/doc.md` with `docts`.
  - The architecture doc covers the Settings route shell, admin-only tabs,
    local preferences, server-backed draft settings, LLM validation, runtime
    info, cloud hooks, and the browse-scope vs working-repository split.
  - Validation: `make web-test` passed.
- Step 3 batch 3 complete:
  - Added `web/src/features/manage/doc.ts` and generated
    `web/src/features/manage/doc.md` with `docts`.
  - Added `web/src/features/upload/doc.ts` and generated
    `web/src/features/upload/doc.md` with `docts`.
  - The Manage architecture doc covers `/manage` composition, upload embedding,
    repository cards, repository-scoped maintenance actions, cloud import, and
    library-wide people rebuild behavior.
  - The Upload architecture doc covers `UploadProvider`, queue state,
    `useUploadProcess`, hash/upload pipelining, batch/chunk transport,
    server-authoritative upload config, global queue status, and the working
    repository boundary.
  - Validation: `make web-test` passed.
- Step 3 batch 4 complete:
  - Added `web/src/features/studio/doc.ts` and generated
    `web/src/features/studio/doc.md` with `docts`.
  - Added `web/src/features/monitor/doc.ts` and generated
    `web/src/features/monitor/doc.md` with `docts`.
  - The Studio architecture doc covers the `/studio` route state machine,
    local recent-edit history, sidecar save semantics, worker-rendered develop
    previews/exports, border tool layering, and non-destructive editing
    decisions.
  - The Monitor architecture doc covers the admin-only route gate, queue/ML/
    capabilities tabs, five-second polling surfaces, queue diagnostics,
    repository-scoped ML coverage, and task-scoped rebuild actions.
  - Validation: `make web-test` passed.
- Step 3 batch 5 complete:
  - Added `web/src/features/home/doc.ts` and generated
    `web/src/features/home/doc.md` with `docts`.
  - The Home architecture doc covers the `/` dashboard composition, URL-backed
    gallery/stats view state, browse-scope repository behavior, featured-photo
    selection, statistics cards, paginated map points, location clusters, and
    the distinction between browse scope and upload's working repository.
  - Validation: `make web-test` passed.
- Step 4 initial contract/type review complete:
  - Reviewed `web/src/features/share/hooks/useShareLinks.ts` and
    `web/src/features/share/hooks/usePublicShareView.ts` against generated
    `web/src/lib/http-commons/schema.d.ts`.
  - Share endpoints have generated response DTOs for list/create/update/revoke
    and public share metadata/assets; the current Chinese sharing docs do not
    rely on guessed fields.
  - `useShareLinks` still casts create/update/revoke/list results to
    `CreateShareLinkResponseDTO`, `ShareLinkDTO`, and `ShareLinkDTO[]`.
    Because the schema already has those payload types, this is a frontend
    type-cleanup candidate rather than a backend DTO annotation gap.
  - `usePublicShareView` casts the public asset infinite-query result to
    `UseInfiniteQueryResult<InfiniteData<PublicShareAssetListResponseDTO>,
    unknown>` and casts pages when flattening assets/totals. This should be
    revisited if the API client can expose the generated page type directly.
  - Adjacent review found `web/src/features/home/hooks/usePhotoStats.ts`,
    `useMapPhotoAssets.ts`, `useLocationClusters.ts`, and
    `useFeaturedPhotos.ts` using similar casts around generated responses.
    These do not block the feature docs, but should be tracked as a separate
    typed-query cleanup if the project wants to enforce the "no API response
    casts" rule strictly.
- Step 5 reader test pass complete:
  - Re-read the Chinese feature docs against the planned reader questions for
    share management, copy-once URLs, Liked removal, Tags vs asset tag filters,
    Folders vs albums, share revocation, and repository-scoped vs library-wide
    maintenance actions.
  - Fixed `site/docs/zh-cn/user-manual/features/home.md` to say Home statistics
    and map data use the selected browse scope/current repository range, not
    upload's working repository.
  - Fixed `site/docs/zh-cn/user-manual/features/assets.md` icon prose that used
    raw Vue-like component text (`<Search />`, `<Plus />`), removing the
    previous VitePress Lucide SSR errors.
  - Validation: `pnpm vitepress build` passed. The previous `assets.md` Lucide
    SSR errors are gone; the build still prints third-party VueUse/Rollup
    PURE-comment warnings.

## Final Coverage Status

User-facing Chinese docs are now present or updated for the route-level gaps
found in the first pass:

| Area | Final status |
| --- | --- |
| Utilities hub, Liked, Folders, Tags, Trash, Duplicates, smart classifiers | Covered in `utilities.md`; linked from `collections.md`. |
| Owner and public sharing | Covered in `sharing.md`; linked from utilities, collections, assets, and albums docs. |
| People list/detail/correction/share flows | Covered in `people.md`; linked from `collections.md`. |
| Lumilio Agent route | Covered in `lumilio.md`; linked from sidebar and assets docs. |
| Assets, Collections, Albums drift | Updated for current sharing and utility behavior. |
| Home scope wording | Updated to use browse scope/repository range instead of upload working repository. |

Feature architecture docs added in this plan:

| Feature | `doc.ts` status |
| --- | --- |
| `lumilio` | Added and generated. |
| `settings` | Added and generated. |
| `manage` | Added and generated. |
| `upload` | Added and generated. |
| `studio` | Added and generated. |
| `monitor` | Added and generated. |
| `home` | Added and generated. |

Remaining explicit deferrals:

- `auth`: public onboarding/account-flow docs are out of this feature-doc pass;
  an architecture `doc.ts` is still a reasonable future candidate if auth flows
  are actively changing.
- `users`: still covered as a Settings admin tab; no separate feature doc is
  needed unless user management becomes a standalone feature.
- `updates` and `portfolio`: routes remain commented out, so they stay
  internal/dormant.
- API response casts in Share/Home hooks: schema DTOs exist, but hooks still use
  casts around generated API responses. Treat as a separate typed-query cleanup,
  not a documentation blocker.

## Work Plan

### 1. Build The Coverage Matrix

Create a read-only matrix before editing:

- Routes from `web/src/routes/routes.tsx`.
- Feature directories from `web/src/features`.
- Existing `web/src/features/*/doc.ts`.
- Existing Chinese feature pages and `zh-cn` sidebar entries.

Classify every feature/route as:

- `covered`: docs exist and match code at a high level.
- `stale`: docs exist but miss current behavior.
- `missing-user-doc`: user-visible feature lacks Chinese user docs.
- `missing-doc-ts`: feature has enough architectural surface to need `doc.ts`.
- `internal-only`: no public feature page needed.

Expected early priorities:

- `share`: missing Chinese user doc.
- `collections/utilities`: stale utility list and child routes.
- `liked`, `folders`, `tags`, `shared-links`: missing or under-documented
  utility pages.
- `people`: detail/correction/share behavior likely under-documented.
- `lumilio`: user-facing route exists, current assets doc only has a placeholder
  reference.

### 2. Fix User Manual Drift

Update or add Chinese Markdown pages in `site/docs/zh-cn/user-manual/features/`.
Start with features that are already user-visible in navigation or routes.

Recommended order:

1. `utilities.md`: make the utility hub match current shortcuts.
2. Add or integrate docs for Liked, Tags, Folders, and Shared Links.
3. Add a sharing page covering create, copy-once token behavior, public viewer,
   revocation, extension, and deletion.
4. Update `collections.md` to describe folders, tags, Liked, shared links, and
   the current relationship between collections and people detail.
5. Update `albums.md` and people-related docs for whole-collection share and
   selected-item share.
6. Add or update Lumilio Agent docs if the route is intended to be public in the
   current milestone.

When adding a new public page, update
`site/docs/.vitepress/sidebar/zh-cn.ts` so it is discoverable.

### 3. Audit Feature `doc.ts` Coverage

For each feature without `doc.ts`, decide whether it has stable architecture
worth documenting.

Likely candidates:

- `settings`: settings tabs, draft state, browse/working repository hooks, AI
  and cloud settings boundaries.
- `manage`: upload + repository maintenance composition, per-repository actions.
- `studio`: home/editor split, worker/WASM edit path, border/develop tools.
- `monitor`: queue, ML, capability polling surfaces.
- `lumilio`: chat store, context contributors, widgets, board layout, mention
  and slash systems.
- `auth`: bootstrap gates, auth provider, MFA/passkey flows.
- `upload`: upload provider/process state and working repository ownership.
- `home`: dashboard data sources and scope behavior.

Skip or defer features that are dormant, trivial wrappers, or not user-visible
unless their architecture is actively changing.

### 4. Check Contract And Type Red Flags

Search feature code for casts around OpenAPI responses and document-impacting
type gaps.

Focus first on:

- `web/src/features/share/hooks/useShareLinks.ts`.
- `web/src/features/share/hooks/usePublicShareView.ts`.
- Any feature docs that would need to describe fields currently accessed through
  `as` casts.

If a cast masks stale or incomplete OpenAPI output, make a separate
implementation plan before changing backend annotations or running `make dto`.
Do not paper over contract drift in documentation.

### 5. Reader Test The Result

After each user-doc batch, test the docs with questions a new user would ask:

- Where do I manage existing share links?
- Why can I only copy a public share URL once?
- How do I browse all liked assets and remove items from Liked?
- What is the difference between tag filtering in Assets and the Tags page?
- What are Folders, and why are they not editable albums?
- Does deleting an album delete original media?
- Does deleting an asset permanently delete the original file?
- Which actions are repository-scoped and which are library-wide?

Fix any ambiguity before moving to the next batch.

## Validation

For docs-only changes:

```bash
cd site/docs && pnpm vitepress build
```

If `doc.ts` files change, regenerate their sibling `doc.md` files using the
documented `docts` render command from `web/`, then run:

```bash
make web-test
```

If frontend code changes become necessary, use the normal frontend gate:

```bash
make web-test
```

If backend API annotations or DTOs change, regenerate contracts:

```bash
make dto
make web-test
```

## Completion Criteria

- Every user-visible route is either documented in Chinese or explicitly
  classified as internal/deferred.
- Current utility shortcuts and collection routes are accurately reflected in
  Chinese docs.
- Sharing behavior is documented from both owner and recipient perspectives.
- Existing public docs no longer contradict current code.
- Any new architecture docs follow the `doc.ts` convention and pass `make
  web-test`.
- Any OpenAPI/type issues found during the review are either fixed or captured
  as separate implementation work with owner files.

## Critical Files For Implementation

- `web/src/routes/routes.tsx`
- `web/src/features/collections/components/utilityShortcuts.ts`
- `web/src/features/share/doc.ts`
- `site/docs/zh-cn/user-manual/features/utilities.md`
- `site/docs/.vitepress/sidebar/zh-cn.ts`
