# Decision: Every public Problem carries an executable recovery entry, and architecture checks keep the storage boundary from growing back

Status: implemented, 2026-10-26 (storage plan P7, PR #262). Companion of
[the storage command boundary](2026-10-26-storage-command-boundary.md).

## Problem

Error presenters used to invent remedies: a "relocate" hint with no command
behind it, an "Open" offered before the primary readiness gate, a generic
"something failed" with no next step. R10 of the storage plan required that
every Problem the API can return names a recovery that can actually be
followed by that actor in that phase, and that the one-classifier, no
handler-built-actions and no Desktop-marker-parsing rules of the refactor
cannot silently regress.

## Decision

- `problem.RecoveryEntry` (actor, phase, kind, route or host action) is part
  of every registered Problem type; `mustRegistry` validates the table at
  start, so a Problem without an entry, or an entry whose route or host
  action does not exist, fails the Server. The kinds are retry, sign-in,
  route, host action, operator and none.
- `problemcatalog` generates the same table into
  `web/src/lib/http-commons/problemRecovery.ts` (a `verify:generated`
  input) and into the Problem pages of the site; the Web's
  `ProblemRecoveryLink` renders the entry next to a refusal, so no
  presenter writes its own hint.
- `architecturecheck` enforces, each rule with a deliberate-violation test:
  every registry route exists in `routes.tsx` and every host action in
  `host_action.go`; no risk code is spelled outside the classifier files;
  no Desktop Go code names a marker file or decodes YAML (the Desktop asks
  `app.ReviewDefaultStorageMove`); no handler spells an action list; the
  candidate files hold neither their own risk warnings nor the legacy
  confirmation gate; every code of the closed vocabularies (risks, action
  blockers and alternatives, Repository and Location actions, candidate
  classifications, recovery classes and dispositions) has a literal entry in
  its Web or Desktop copy module.

## Alternatives considered

- **Free-text recovery hints in each presenter.** Rejected: they drift from
  the routes and commands that exist, and nothing fails when a route is
  renamed.
- **A review-time checklist instead of checks.** Rejected: the plan's own
  history shows classifiers and action lists reappear in the first PR that
  is in a hurry; the checks cost one test each and fail at the right place.
- **A Desktop-side marker reader for the default Storage Location.**
  Rejected: it was the second classifier of the marker format; the Server
  facade answers with typed errors and the Desktop only presents.

## Where it lives

`server/internal/api/problem`, `server/tools/problemcatalog`,
`server/tools/architecturecheck` (`recovery_reachability.go`, `p7_checks.go`),
`web/src/components/problem`, `web/src/lib/http-commons/problemRecovery.ts`.
