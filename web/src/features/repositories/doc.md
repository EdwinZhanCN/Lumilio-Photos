# Repositories

Repositories owns the shared repository option contract, repository-aware
scope selection, scan lifecycle, and the repository management UI composed
by Manage. Consumers use the feature's root public entry; internal paths are
not public API.

## Data

[useRepositoryOptions](./api/useRepositoryOptions.ts) reads the server's active repository list.
[normalizeRepositoryOptions](./model/repositoryOptions.ts) is the React-free DTO adapter that
produces [RepositoryOption](./types.ts) values for every consumer.
[useRepositoryRoots](./api/useRepositoryRoots.ts) reads the admin-visible Storage Locations that
the native Desktop host has authorized; Web callers receive identities and
reachability, never an arbitrary host-path write capability.
[useRepositoryScan](./api/useRepositoryScan.ts) starts repository scans and stack detection, while
[waitForRepositoryScan](./api/waitForRepositoryScan.ts) follows the requested scan run to a terminal
state before repository-aware queries are invalidated.

## Flows

[BrowseScopeSelect](./flows/browse-scope/BrowseScopeSelect.tsx) and [useBrowseScope](./flows/browse-scope/useBrowseScope.ts) form the browse-scope
flow. An empty preference intentionally means all repositories and is valid
for list, map, collection, and statistics pages.

[useWorkingRepository](./flows/working-repository/useWorkingRepository.ts) forms the upload-target flow. It must resolve a
concrete repository and falls back to the primary or first option that
[isRepositoryUnavailable](./model/repositoryOptions.ts) reports as writable — auto-selecting an
unreachable repository would guarantee the next upload is refused. An
explicit user choice is left alone even when it goes offline. Browse scope
and working repository remain separate preferences because their empty-state
semantics differ.

## Reachability

[RepositoryStatus](./types.ts) carries a repository's reachability alongside its
activity. An `offline` repository is one whose folder is not currently
mounted — an unplugged external drive — while `error` means the on-disk
identity or config needs attention. Both stay selectable as browse filters
and are refused as write targets. An unrecognized status normalizes to
`active`, because a client-side guess must not incorrectly block a valid
repository.

[RepositoryGrid](./flows/manage/RepositoryGrid.tsx) owns the repository-management surface. Its create
modal selects a registered root and sends explicit storage-layout and
duplicate-filename policies through [useCreateRepository](./api/useCreateRepository.ts); local and
cloud creation share the same body, with only the cloud credential differing.
Auth setup uses the same public mutation for primary-repository creation.
[isStorageStrategy](./model/repositorySetup.ts) keeps storage-policy parsing in the repository
model. The grid receives maintenance commands from the higher Manage
composition route rather than importing those domains.

## State

Repository lists are TanStack Query server state. Browse and working ids are
persisted user-scoped preferences through the lower preferences contract and
are cleared by authentication session reset. Scan/detection id sets are
request-local interaction state inside [useRepositoryScan](./api/useRepositoryScan.ts); fetched
repository data is never copied into Context or Zustand.
