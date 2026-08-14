# GitHub Agent Task Loop

Status: active. Workflow, state-ownership, and review contracts were frozen
2026-08-14, and the executor contract was amended the same day to use the
maintainer's subscription-backed Codex Cloud rather than API-billed
`openai/codex-action`. Phase 0 and the first repository slice of Phases 1–2
are implemented and validated on `dev`: 37 offline policy tests, the
path-filtered CI gate, privacy-first Issue Form, Project synchronization
helper, versioned triage prompt/schema, and a managed manual Cloud handoff.
Live activation is
partially configured: personal Project #6 is linked to the repository with the
frozen fields/options, required labels and repository variables exist, and no
built-in Status writer is enabled. `AGENT_PROJECT_TOKEN` is stored, and the
Codex Cloud repository/environment connection passed its first read-only
environment check. Default-branch landing, the saved capture entry, live token
permission verification, and representative pilots remain pending.
`/agent-submit`, `/agent-revise`, and `/agent-retry` are implemented;
`/agent-run` and all repository-write or closing workflows remain disabled,
and Phases 3–5 have not started.

Goal: turn a trusted maintainer's quick GitHub Issue into a resumable,
human-gated agent task: Codex triages the report, records either a compact
Issue contract or a repository-native active exec plan, implements only after
explicit approval, produces reviewable validation evidence, and closes the
Issue only after a human merges the final implementation PR into `dev`.

The loop optimizes capture for bugs noticed while using Lumilio: creating the
initial reminder should take less than 30 seconds and must never require logs,
original media, or a complete reproduction before the Issue can be saved.

## Current baseline (verified 2026-08-14)

- The repository is public. Untrusted users can create Issues, so an
  `issues.opened` event cannot by itself authorize model spend, secret access,
  repository writes, or Project mutations.
- The GitHub default branch is `main`; day-to-day integration is performed
  through `dev`. GitHub closing keywords close an Issue automatically only
  when the linked PR targets the default branch, so a PR merged into `dev`
  needs an explicit, repository-owned closer.
- GitHub resolves `issues`, `issue_comment`, and `workflow_dispatch` workflow
  definitions from the default branch. The orchestration code deliberately
  checks out `dev`, but the workflow files must also land on `main` before
  live Issue events or the manual recovery button can activate them.
- [CI](../../../../../.github/workflows/ci.yml) runs for every pull request,
  regardless of base branch. Agent-created pull requests targeting `dev` can
  therefore use the existing required gates.
- The `dev` slice contains the Quick task form, `agent-intake.yml`, a triage
  prompt/schema, and testable policy, GitHub API, Project, and publisher
  helpers. The Issue entrypoints are not active on GitHub until their workflow
  definitions land on the default branch and the external setup below is
  complete.
- The repository already has the memory and procedure homes required by this
  loop: [agent-harness.md](../../agent-harness.md),
  [lumilio-exec-plan](../../../../../.agents/skills/lumilio-exec-plan/SKILL.md),
  and [lumilio-select-checks](../../../../../.agents/skills/lumilio-select-checks/SKILL.md).
  These harness changes must be present on `dev` before a remote runner is
  allowed to rely on them.
- Subscription-backed Codex Cloud is authenticated with the maintainer's
  ChatGPT account. The documented `openai/codex-action` GitHub Action instead
  requires an API key and uses API billing, so it is intentionally absent from
  this loop.
- OpenAI does not document a personal-subscription endpoint that lets a GitHub
  Action create an arbitrary Codex Cloud task. The handoff is therefore
  explicit: Actions prepare bounded context, and a human starts or continues
  the Cloud task.
- GitHub's separate `openai code agent` integration can be assigned to Issues,
  but it consumes GitHub Copilot AI credits. It is not the executor selected by
  this plan and must not be silently enabled as a substitute.

## Non-goals

- Automatically run an agent for an Issue created by an untrusted actor.
- Automatically approve or merge a pull request, change branch protections,
  or make `dev` the GitHub default branch.
- Treat a GitHub Project, labels, and comments as three independent copies of
  the same state.
- Let the triage run implement code, or let a plan-only PR contain code,
  generated artifacts, configuration, or unrelated documentation.
- Replace the existing exec-plan, decision, postmortem, documentation, test,
  or CI obligations with Issue prose.
- Upload application logs, databases, media paths, original media, secrets,
  or diagnostics automatically from Lumilio.
- Build an arbitrary slash-command framework. The initial protocol contains
  only the commands frozen below.
- Claim that a bot-authored mention or undocumented endpoint can start a
  subscription-backed Codex Cloud task.
- Route tasks through GitHub's Copilot-billed `openai code agent` when the
  maintainer selected ChatGPT subscription usage.

## State ownership (frozen)

Each fact has exactly one owner:

- The Issue body owns the original observation. Agents do not rewrite it.
- For a Small task, one bot-managed **Agent Contract** comment owns approved
  scope, non-goals, todos, validation boundaries, and the expected memory.
- For Exec Plan work, the file under `exec-plans/active/` owns the execution
  contract after its plan PR is merged. The Issue comment links to it rather
  than duplicating it.
- The GitHub Project `Status` field owns workflow position. State labels are
  forbidden.
- Labels classify enrollment, type, and area only.
- Pull requests own diffs, review discussion, checks, and merge evidence.
- The Issue open/closed state owns whether the task as a whole remains live.

Actions are the only writer of Project workflow status. Agents may recommend
a state in structured output, but they do not call the Project API directly.

## GitHub Project contract (frozen)

Create one personal Project for the repository with these exact fields and
options:

| Field | Type | Values |
| --- | --- | --- |
| `Status` | single select | `Inbox`, `Triaging`, `Needs input`, `Human review`, `Ready`, `Running`, `PR review`, `Blocked`, `Done` |
| `Track` | single select | `Small`, `Exec Plan` |
| `Area` | single select | `Server`, `Web`, `Desktop`, `Deploy`, `Docs`, `Cross-cutting`, `Unknown` |
| `Priority` | single select | `P0`, `P1`, `P2`, `P3` |
| `Agent run` | text | URL of the latest orchestration run |

Repository variables contain the Project owner, number, and base branch;
field and option node IDs are discovered by name at run time and are never
checked into the repository. A missing or renamed required field moves the
task to `Blocked` with a diagnostic comment rather than silently creating a
second field or falling back to labels.

Labels are limited to:

- `quick-capture`: created through the fast Issue form;
- `agent-task`: a trusted actor has enrolled the Issue in this loop;
- `bug` / `enhancement`: task type; and
- `area:web`, `area:server`, `area:desktop`, `area:deploy`, `area:docs`, or
  `area:cross-cutting`: classification.

Project built-in automation may add or archive items, but custom Actions
remain responsible for the frozen intermediate statuses so the state machine
has one deterministic implementation.

### Initial activation checklist (pending)

The repository slice stays inert until a maintainer completes all of these
steps; values are named here, while secret values remain outside the repo:

1. **Done 2026-08-14.** Personal Project #6 has the exact required
   fields/options, is linked to `EdwinZhanCN/Lumilio-Photos`, and its built-in
   Status writers are disabled.
2. **Done 2026-08-14.** The classification labels exist, especially
   `quick-capture` and `agent-task`; GitHub Issue Forms do not create missing
   labels.
3. **Done 2026-08-14.** Repository variables are
   `AGENT_PROJECT_OWNER=EdwinZhanCN`, `AGENT_PROJECT_NUMBER=6`, and
   `AGENT_BASE_BRANCH=dev`.
4. **Done 2026-08-14; live permission check pending.** A classic PAT is stored
   as `AGENT_PROJECT_TOKEN`; GitHub exposes only the Secret name, not its
   value. The first disposable Issue will verify that it can mutate Project
   #6. No OpenAI API key or repository publishing token is part of the
   subscription-backed executor contract.
5. **Done 2026-08-14.** Codex Cloud has access to this repository, and its
   Cloud environment passed a read-only runtime/branch check. Keep
   `AGENT_PROJECT_TOKEN`, OpenAI API keys, and repository-write tokens out of
   that environment; select `dev` when starting each handoff.
6. **Pending.** Land the Issue-event workflow and its checked-in helpers on
   the default branch `main`, while retaining `dev` as the explicit
   checkout/base branch.
7. **Pending.** Save
   `https://github.com/edwinzhancn/Lumilio-Photos/issues/new?template=quick-task.yml`
   in GitHub Mobile or the browser for the sub-30-second capture path.
8. **Pending.** Create one disposable trusted Quick task, then verify the
   label, single Project item, Status transitions, managed comment, and
   absence of duplicate runs. Use the `Agent command` manual `retry` entry to
   exercise recovery.

## Fast Issue intake contract (frozen)

Add one bilingual `Quick task / 快速记录` Issue Form. It asks only for:

1. a concise title;
2. one required textarea describing what happened or what should change;
3. an optional surface selector;
4. optional reproduction notes and evidence; and
5. optional, explicitly sanitized version and environment information.

The form applies `quick-capture`, not `agent-task`. On creation, the intake
workflow verifies the author's repository permission. `OWNER`, `MEMBER`, or
`COLLABORATOR` with effective `write`, `maintain`, or `admin` permission is
trusted; every other Issue remains a normal report and causes no model call.
A trusted quick capture receives `agent-task`, is added to the Project at
`Inbox`, and starts triage. A maintainer can enroll an external report later
by applying `agent-task`; that label event performs the same authorization
check against the actor applying it.

The initial form and any future in-app link must warn against attaching
original media, secret values, database files, or unsanitized logs. A future
in-app `Report a problem` entry may prefill version, platform, and route, but
it is not required by the first implementation phase.

## Comment command protocol (frozen)

The commands are plain comment conventions parsed by Actions, not registered
GitHub slash commands. Only the first non-empty line is a command; a command
must match exactly and the remaining text is feedback or a structured Cloud
result. No command causes a GitHub Action to call a model:

- `/agent-submit` — validate the JSON-only result copied back from a manually
  started Codex Cloud preflight, then publish the managed contract and Project
  classification.
- `/agent-revise` — prepare a revised Cloud handoff containing the trusted
  feedback and current Issue context. The maintainer starts or continues the
  Cloud task manually.
- `/agent-run` — approve the latest contract and prepare the next plan or
  implementation Cloud handoff. It remains disabled until the corresponding
  PR publisher and guards land.
- `/agent-retry` — restore a published state or prepare the failed handoff
  again without starting a model automatically.

Commands from bots or actors without `write`, `maintain`, or `admin`
permission are ignored without preparing or publishing a handoff.
`/agent-run` is rejected when the Issue is `Needs input`, no contract exists,
an Exec Plan PR has not been
merged into `dev`, another run holds the Issue concurrency lock, or the Issue
is already closed.

Every managed Issue and PR contains a hidden, versioned marker. The marker
records the Issue number, artifact role, contract revision, and whether an
implementation PR is final. It is data for deterministic Actions code; Codex
cannot select a different Issue or mark an intermediate PR final merely by
printing a marker in its response.

## Triage contract (frozen)

Triage is a read-only Codex Cloud task started by a trusted maintainer against
the latest `dev`. The managed handoff comment contains the bounded prompt and
explicit repository/base-branch selection. The task reads root `AGENTS.md`,
the system map, active plans, the harness, and only the module references
relevant to the Issue. It checks for related active plans and open Issues, but
it may only recommend that a report is a duplicate; it cannot close it.

The final output is validated against a checked-in JSON schema:

```json
{
  "track": "small | exec_plan | needs_input",
  "summary": "string",
  "area": "server | web | desktop | deploy | docs | cross-cutting | unknown",
  "priority": "P0 | P1 | P2 | P3",
  "scope": ["string"],
  "non_goals": ["string"],
  "todos": ["string"],
  "validation": ["observable outcome"],
  "memory": "none | decision | postmortem | active_plan",
  "questions": ["string"],
  "related": ["issue, PR, plan, or path"]
}
```

`Small` means one implementation PR, no multi-phase delivery, no migration or
recovery coordination, and no contract that must be frozen before code.
`Exec Plan` means multi-phase or multi-PR work, a cross-module contract, or a
migration/recovery that later sessions must be able to resume. A non-trivial
single-PR task remains Small but still records the required decision or
postmortem in its implementation PR. Uncertainty that changes the intended
scope produces `needs_input`, never an optimistic Small classification.

The Cloud task returns JSON only. A trusted maintainer copies that response
into an Issue comment following `/agent-submit`; the publisher converts the
validated JSON into either the managed Agent Contract or the plan-generation
input. Invalid JSON, missing fields, or an unknown enum value moves the task to
`Blocked`. Returned text is bounded by the GitHub comment limit and is never
treated as shell source.

## Small track contract (frozen)

The intake workflow creates or updates exactly one comment marked
`<!-- lumilio-agent-contract:v1 -->` with:

```markdown
## Scope
## Non-goals
## Todos
## Validation
## Documentation / Memory
## Open questions
```

`/agent-revise` prepares a new Cloud preflight handoff; the following
validated `/agent-submit` increments the contract revision and updates that
comment in place. `/agent-run` captures the approved revision in the
implementation PR marker. If a later revision changes the contract while
implementation is running, the existing run cannot publish a PR against the
stale revision.

## Exec Plan track contract (frozen)

The maintainer starts the plan task from the latest `dev` in Codex Cloud and
publishes a Draft PR titled `plan: #<number> <summary>`. Codex Cloud owns the
branch name; automation identifies the artifact through its Issue, role, and
contract-revision marker rather than assuming an undocumented branch prefix.
The PR body uses `Refs #<number>` and must not contain a closing keyword.

The plan-generation job may change exactly one new file:

`site/docs/internal/exec-plans/active/issue-<number>-<slug>.md`

The file follows `lumilio-exec-plan`; its Status line identifies the linked
Issue and records which contracts the first human review freezes.
A deterministic publisher rejects the patch if any other path changes, the
plan is a symlink, the path escapes the active-plan directory, the Issue
number does not match, or the required plan sections are absent.

`/agent-revise` on the Issue or plan PR continues the same Cloud task and
updates the same PR, while remaining subject to the one-file allowlist. Human
Review #1 completes only when a human merges the plan-only PR into `dev`. The
merge moves the Issue to `Ready` but does not close it. `/agent-run` then
starts a separate implementation task from the new `dev`; continuing
implementation inside the unmerged Plan PR is forbidden because an active
plan must be available to later sessions and updated as implementation PRs
land.

Multi-PR work keeps the Issue open and updates the active plan Status and
phase list in each implementation PR. The final implementation PR verifies
the plan boundaries, extracts durable decisions, moves surviving debt,
updates owning references, and deletes the active plan as required by
`lumilio-exec-plan`.

## Implementation and PR contract (frozen)

An implementation run:

1. starts from the latest `dev`, never the GitHub default chosen implicitly;
2. reads the approved Small contract or merged active plan and its exact
   revision;
3. follows every applicable repository skill and module reference;
4. implements tests, gates, generated artifacts, documentation, and the
   required memory in the same diff;
5. selects the narrowest local checks through `lumilio-select-checks`;
6. creates or updates one Cloud-managed implementation branch and PR; and
7. opens a PR targeting `dev`, with the Issue and contract revision in a
   hidden marker and a human-readable validation summary.

The PR body uses `Implements #<number>` rather than a GitHub closing keyword.
A Small PR is final. An Exec Plan PR is final only when the plan's validation
boundaries are satisfied and the plan is removed; intermediate phase PRs are
marked non-final. Only final implementation PRs can close the Issue.

When a human requests changes, `/agent-run` on the implementation PR
continues the same Cloud task, reuses the same PR, and incorporates the
trusted command comment plus current review state. It does not create a
second PR. The agent may never approve or merge its own PR. Human Review #2
ends only with a human merge into `dev`.

## Merge and close contract (frozen)

The closer runs on a merged pull request and requires all of:

- base branch is exactly `dev`;
- the PR carries a publisher-generated agent marker;
- artifact role is `implementation`;
- `completion=final`; and
- the referenced Issue is open and carries `agent-task`.

It closes the Issue with a short comment linking the merged PR, then sets the
Project Status to `Done`. Merging a plan-only or intermediate implementation
PR never closes the Issue. Closing an Issue manually sets `Done`; reopening it
returns it to `Human review` without starting Codex automatically.

The closer is idempotent: repeated events, retries, an already-closed Issue,
or an already-`Done` Project item succeed without duplicate comments.

## Executor and privilege contract (frozen)

The first implementation uses Codex Cloud authenticated by the trusted
maintainer's ChatGPT account. GitHub Actions never receive an OpenAI credential
and never call a model. The workflow interface remains repository-owned prompt
files plus versioned JSON schemas, but there is an explicit human handoff:

1. Actions authorize the event, bound and sanitize Issue context, and publish
   one managed Cloud handoff comment.
2. A maintainer opens Codex Cloud, selects this repository and `dev`, and
   starts or continues the task with that prompt.
3. Read-only preflight returns JSON only. The maintainer posts it after
   `/agent-submit`; Actions validate it before publishing any contract or
   Project classification.
4. Later plan and implementation handoffs instruct Codex Cloud to create or
   update the exact guarded PR. PR-event workflows validate markers, target
   branch, diff shape, and state transitions before accepting the result.

The manual start is a trust and billing boundary, not a temporary fake API.
No workflow makes a bot-authored Codex mention, exports local ChatGPT
credentials, or calls an undocumented cloud-task endpoint. GitHub's separate
Copilot-billed `openai code agent` is outside this plan.

Credential contract amended 2026-08-14 after verification against GitHub's
current user-Project API: a user-owned Project cannot use a fine-grained PAT.
Project GraphQL mutations therefore use a classic PAT with only the `project`
scope, stored as `AGENT_PROJECT_TOKEN`. Normal Issue/PR writes use the
workflow-scoped `GITHUB_TOKEN`; the Project PAT is never placed in a Codex
Cloud environment or model-facing prompt. Moving the board to an
organization-owned Project would allow a GitHub App to replace the Project PAT
later without changing the workflow protocol.

Issue bodies, titles, comments, PR reviews, commit messages, and attached text
are untrusted prompt data. Workflows pass them through environment or JSON
files, never interpolate them into shell source, remove hidden HTML from the
model-facing copy, impose size limits, and tell Codex that repository and
workflow instructions outrank Issue content. Deterministic publisher guards,
not the model's self-report, enforce write scope.

Only the trusted maintainer's explicit Cloud start can consume subscription
usage. Repository permission checks still gate enrollment, handoff generation,
structured result publication, and Project mutation. Each Issue has a
non-cancelling concurrency group so two commands cannot publish concurrently.
Every transition records the Action run URL. Failures move to `Blocked` once,
post one diagnostic, and wait for `/agent-retry` or `/agent-revise`; they do
not loop automatically.

## Execution phases

### Phase 0 — Lock the control-plane failures

Before enabling any model call or external write, add deterministic fixtures
and tests for the failure modes that would make this loop unsafe or noisy:

1. exact command parsing, including leading whitespace, feedback text,
   lookalike commands, bot authors, and comments on unrelated PRs;
2. trusted/untrusted actor decisions and the rule that an untrusted Issue
   cannot reach the Codex step;
3. Project state transition validity and idempotent retries;
4. triage JSON validation and rendering of the managed Small comment;
5. rejection of a plan patch with a second file, path traversal, symlink,
   wrong Issue number, or missing required section;
6. rejection of stale contract revisions and duplicate active runs; and
7. closer behavior for plan, intermediate, final, wrong-base, repeated, and
   manually closed/reopened cases.

Implementation evidence (2026-08-14): `task agent-loop:test` passes the 37
offline control-plane tests. A deliberate direct guard invocation containing
one allowed plan plus `server/cmd/main.go` exited non-zero with
`plan patch must contain exactly one changed path`; the committed regression
test asserts the same rejection while the normal gate remains green.

Repository publication evidence (2026-08-14): `task architecture:check`,
`task server:test`, `task web:test`, `task verify:generated`,
`task compose:test`, and `task ci:site` pass. The isolated E2E stack also
passed `task web:test:video-semantic` (2 tests) in default replay mode and was
torn down with its temporary volumes afterward.

Prove the plan-only guard can fail by running a fixture with one allowed plan
file plus one forbidden file and recording the red run in the PR.

Put parsers, schemas, renderers, and policy functions in testable checked-in
files rather than large inline `github-script` bodies. Add the narrowest root
Task target and CI path-filter wiring justified by these files, following
[lumilio-add-task-target](../../../../../.agents/skills/lumilio-add-task-target/SKILL.md).

Exit: all state and publishing policies can be tested locally without a
GitHub token, Project, OpenAI credential, network call, or live Issue.

### Phase 1 — Quick capture and Project state

1. Create the personal Project and exact fields/options.
2. Configure repository variables for Project owner/number and `dev` base.
3. Create the classic Project token and store `AGENT_PROJECT_TOKEN`. Keep all
   OpenAI credentials out of GitHub Actions and Codex-facing prompt context.
4. Add the bilingual Quick task Issue Form and privacy warning.
5. Add deterministic enrollment, Project synchronization, status comments,
   and a manual `workflow_dispatch` recovery entry.
6. Add a saved GitHub Mobile/browser URL so a maintainer can create the form
   in less than 30 seconds.

Exit: a trusted quick capture appears once in the Project, passes through
`Inbox`, reaches `Triaging`, and has one managed Cloud handoff; an untrusted
submission creates no agent enrollment, handoff, or privileged Project
mutation; Project failures become visible and retryable.

### Phase 2 — Read-only preflight and Small contracts

1. Add versioned triage prompt and output schema.
2. Publish one bounded handoff for a trusted maintainer to start a read-only
   Codex Cloud preflight against `dev`; record the handoff Action URL.
3. Accept the JSON-only result through `/agent-submit` and validate it before
   applying Track, Area, Priority, or Status.
4. Render or update the single Small Agent Contract comment.
5. Implement `Needs input`, `/agent-submit`, `/agent-revise`, and
   `/agent-retry` without any repository write or automated model call.
6. Pilot at least five representative tasks: local UI bug, backend bug,
   generated API contract change, cross-module feature, and insufficient
   report. Review false Small/Exec Plan decisions before enabling plan writes.

Exit: the preflight cannot modify code; every trusted Issue reaches a
reviewable Small contract, a validated plan request, `Needs input`, or one
visible `Blocked` state, with no duplicate comments or automated model runs.

### Phase 3 — Plan-only Draft PRs

1. Add the plan-generation prompt and managed Codex Cloud handoff.
2. Have the maintainer start or continue the Cloud task; validate its PR with
   the one-file active-plan guard before accepting the transition.
3. Create/reuse the Cloud-managed plan PR targeting `dev` through Codex
   Cloud's connected-repository publishing path.
4. Implement `/agent-revise` against the same PR and contract revision.
5. On human merge, move the Issue to `Ready` without closing it.
6. Verify normal CI is triggered for the generated PR without exposing the
   Project token or requiring an agent to approve its own workflow.

Exit: the generated Draft PR contains exactly one valid active plan; malicious
Issue text and deliberately malformed patches cannot expand the diff; merging
the plan makes it readable from `dev` and leaves the Task Issue open.

### Phase 4 — Implementation and review loop

1. Add the implementation prompt and `/agent-run` state gate.
2. Prepare the guarded implementation handoff and have the maintainer start or
   continue the Codex Cloud task against the approved revision.
3. Run the checks selected by `lumilio-select-checks`; preserve CI as the
   authoritative platform-wide gate, especially for macOS and Windows.
4. Create/reuse one implementation PR targeting `dev`, with contract,
   validation, documentation, and memory evidence in its body.
5. Support trusted PR follow-up comments through `/agent-run` without
   creating a second PR.
6. For Exec Plan tasks, update plan status in intermediate PRs and enforce
   decision extraction plus plan deletion in the final PR.

Exit: both a Small task and a multi-phase Exec Plan task can move from human
approval to a CI-backed implementation PR; agent revisions remain on the same
PR; no agent can approve or merge the result.

### Phase 5 — Merge closure, observability, and handoff

1. Add the `dev`-aware final-implementation closer and Project `Done`
   transition.
2. Add concise handoff and PR links for `Running`, `Blocked`, and `PR review`;
   retain returned preflight output only in its bounded trusted submission.
3. Exercise cancellation, retry, stale revision, Project outage, Codex
   failure, CI failure, closed PR, and reopened Issue recovery.
4. Write `lumilio-agent-task-loop` as the recurring procedure skill and route
   it from root `AGENTS.md` and `agent-harness.md`.
5. Document one-time GitHub Project, repository variable, secret, and saved
   reply setup without copying secret values into the repository.
6. Complete this plan: verify every boundary below, extract durable workflow
   decisions into `.agents/decisions/`, move surviving debt to the tracker,
   update owning references, and delete this file.

Exit: the full loop closes only after a human merge into `dev`, is recoverable
from every expected failure state, and leaves a durable skill and decision
record rather than an active plan archive.

## Validation boundaries

The plan is complete only when all of these observable scenarios pass:

- A trusted maintainer creates a useful quick Issue from a saved mobile or
  browser entry in under 30 seconds without supplying logs or media.
- An untrusted public Issue and an unauthorized command produce zero Codex
  handoffs, zero repository writes, and zero privileged Project mutations.
- The managed prompt is started manually in subscription-backed Codex Cloud;
  triage reads repository guidance and classifies representative Small, Exec
  Plan, and Needs-input cases correctly. Invalid `/agent-submit` JSON is
  blocked before contract or classification mutation.
- Small triage maintains one versioned Agent Contract comment; revision does
  not duplicate it, and stale approved revisions cannot publish.
- An Exec Plan Draft PR targets `dev`, contains exactly one new active plan,
  and the path guard demonstrably rejects one extra file.
- Human Review #1 can revise the plan repeatedly; merging the Plan PR makes
  the plan visible on `dev`, moves the Issue to `Ready`, and does not close it.
- `/agent-run` creates or reuses one Cloud-managed implementation PR from the
  latest `dev`; the PR contains tests, selected check evidence, documentation,
  generated artifacts, and the required memory for its actual diff.
- PR feedback followed by `/agent-run` updates the same PR and cannot act on
  another Issue, branch, or contract revision.
- An intermediate Exec Plan PR updates the live plan and leaves the Issue
  open. The final PR satisfies validation boundaries, extracts durable
  decisions, and deletes the plan.
- Opening or updating an agent PR triggers the existing CI workflow without
  exposing `AGENT_PROJECT_TOKEN` to Codex Cloud.
- Merging a Plan PR, non-final implementation PR, PR to a branch other than
  `dev`, or ordinary human PR does not close a Task Issue.
- A human merge of the final implementation PR into `dev` closes exactly the
  linked Issue and moves exactly its Project item to `Done`; replaying the
  event is idempotent.
- A failed Cloud task, failed publisher, Project outage, or failed CI reaches
  one visible recoverable state and never loops model spend automatically.
- No workflow uploads original media, unsanitized logs, databases, secret
  values, or local filesystem paths as Issue comments or public artifacts.

## Expected repository surfaces

- `.github/ISSUE_TEMPLATE/quick-task.yml`
- `.github/ISSUE_TEMPLATE/config.yml`
- `.github/workflows/agent-intake.yml`
- `.github/workflows/agent-command.yml`
- `.github/workflows/agent-close.yml`
- `.github/codex/prompts/{triage,plan,implement}.md`
- `.github/codex/schemas/triage.schema.json`
- `.github/codex/scripts/` and fixture tests for parsing, state, rendering,
  publishing guards, and closing policy
- root Taskfile plus `.github/workflows/ci.yml` for the earned local/CI gate
- `.agents/skills/lumilio-agent-task-loop/SKILL.md`
- root `AGENTS.md`, `site/docs/internal/agent-harness.md`, and contributor
  documentation for routing and one-time setup
- `.agents/decisions/` when this plan completes; this active plan is then
  deleted rather than archived
