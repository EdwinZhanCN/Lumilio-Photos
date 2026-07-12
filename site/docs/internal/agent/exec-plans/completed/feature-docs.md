# Feature Documentation Review

Status: completed.

## Result

- Updated Chinese feature docs for assets, collections, albums, utilities, and
  Home repository-scope wording.
- Added Chinese docs for sharing, people correction, and Lumilio, with sidebar
  entries and cross-links.
- Added generated architecture docs for `home`, `lumilio`, `manage`, `monitor`,
  `settings`, `studio`, and `upload` using the `doc.ts` convention.
- Reader-tested Liked, Folders, Tags, sharing/revocation, people correction,
  and browse-scope versus working-repository explanations against the code.

## Validation record

`make web-test` passed after each architecture-doc batch. The VitePress build
passed; the earlier Lucide SSR errors in `assets.md` were removed. Remaining
third-party VueUse/Rollup warnings were non-blocking.

## Deferred outside this plan

- `auth` may gain a `doc.ts` when those flows next change.
- `users` remains documented as part of Settings.
- `updates` and `portfolio` remain dormant while their routes are disabled.
- Generated DTOs exist for the reviewed Share/Home APIs, but several hooks
  still cast response values. That is typed-query debt, not documentation drift.
