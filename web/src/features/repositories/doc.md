# Repositories

Repositories owns the shared repository option contract, repository-aware
scope selection, scan lifecycle, and the repository management UI composed
by Manage. Consumers use the feature's root public entry; internal paths are
not public API.

## Data

[useRepositoryOptions](./api/useRepositoryOptions.ts) reads the server's active repository list.
[normalizeRepositoryOptions](./model/repositoryOptions.ts) is the React-free DTO adapter that
produces [RepositoryOption](./types.ts) values for every consumer.
[useRepositoryScan](./api/useRepositoryScan.ts) starts repository scans and stack detection, while
[waitForRepositoryScan](./api/waitForRepositoryScan.ts) follows the requested scan run to a terminal
state before repository-aware queries are invalidated.

## Flows

[BrowseScopeSelect](./flows/browse-scope/BrowseScopeSelect.tsx) and [useBrowseScope](./flows/browse-scope/useBrowseScope.ts) form the browse-scope
flow. An empty preference intentionally means all repositories and is valid
for list, map, collection, and statistics pages.

[useWorkingRepository](./flows/working-repository/useWorkingRepository.ts) forms the upload-target flow. It must resolve a
concrete repository and falls back to the primary or first option that
[isRepositoryOffline](./model/repositoryOptions.ts) reports as reachable — auto-selecting an
unreachable repository would guarantee the next upload is refused. An
explicit user choice is left alone even when it goes offline. Browse scope
and working repository remain separate preferences because their empty-state
semantics differ.

## Reachability

[RepositoryStatus](./types.ts) carries a repository's reachability alongside its
activity. An `offline` repository is one whose folder is not currently
mounted — an unplugged external drive — and is distinct from one whose data
is gone: it stays selectable as a browse filter while being refused as an
upload target. An unrecognized status normalizes to `active`, because
wrongly reading a repository as offline blocks writes to it.

[RepositoryGrid](./flows/manage/RepositoryGrid.tsx) owns the repository-management surface. Its create
modal delegates server mutation and invalidation to
[useCreateRepository](./api/useCreateRepository.ts); Auth setup uses the same public mutation for
primary-repository creation. [isStorageStrategy](./model/repositorySetup.ts) keeps storage-policy
parsing in the repository model. The grid receives maintenance commands from
the higher Manage composition route rather than importing those domains.

## State

Repository lists are TanStack Query server state. Browse and working ids are
persisted user-scoped preferences through the lower preferences contract and
are cleared by authentication session reset. Scan/detection id sets are
request-local interaction state inside [useRepositoryScan](./api/useRepositoryScan.ts); fetched
repository data is never copied into Context or Zustand.
