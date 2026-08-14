import assert from "node:assert/strict";
import test from "node:test";

import {
  closeDecision,
  commandDecision,
  implementationGate,
  intakeDecision,
  isTrustedActor,
  parseAgentCommand,
  parseTaskMarker,
  renderAgentContract,
  renderTaskMarker,
  sanitizePromptText,
  statusForIssueAction,
  transitionDecision,
  validatePlanPatch,
  validateTriage,
} from "./policy.mjs";

const smallTriage = Object.freeze({
  track: "small",
  summary: "Fix the broken thumbnail action",
  area: "web",
  priority: "P2",
  scope: ["Restore the action on the asset viewer."],
  non_goals: ["Redesign the viewer."],
  todos: ["Add a regression test.", "Fix the handler."],
  validation: ["The action works in Chromium."],
  memory: "postmortem",
  questions: [],
  related: ["web/src/features/assets"],
});

const validPlan = `# Repair task orchestration

Status: active. Linked to Issue #42; Human Review #1 freezes the workflow contract.

Goal: make the task resumable.

## Non-goals

- Do not merge automatically.

## Execution phases

### Phase 0 — Lock the failures

### Phase 1 — Implement the loop

## Validation boundaries

- A human merge remains required.
`;

test("parses only an exact command on the first non-empty line", () => {
  assert.deepEqual(parseAgentCommand("  \n  /agent-revise  \nKeep the scope small."), {
    command: "revise",
    feedback: "Keep the scope small.",
  });
  assert.deepEqual(parseAgentCommand("/agent-run"), { command: "run", feedback: "" });
  assert.deepEqual(parseAgentCommand("/agent-submit\n{\"track\":\"small\"}"), {
    command: "submit",
    feedback: '{"track":"small"}',
  });
  assert.equal(parseAgentCommand("Please run\n/agent-run"), null);
  assert.equal(parseAgentCommand("/agent-run now"), null);
  assert.equal(parseAgentCommand("／agent-run"), null);
  assert.equal(parseAgentCommand("/agent-rerun"), null);
});

test("trust requires a human collaborator and effective write permission", () => {
  assert.equal(
    isTrustedActor({ permission: "write", association: "COLLABORATOR", actorType: "User" }),
    true,
  );
  assert.equal(
    isTrustedActor({ permission: "read", association: "COLLABORATOR", actorType: "User" }),
    false,
  );
  assert.equal(
    isTrustedActor({ permission: "admin", association: "NONE", actorType: "User" }),
    false,
  );
  assert.equal(
    isTrustedActor({ permission: "admin", association: "OWNER", actorType: "Bot" }),
    false,
  );
});

test("untrusted intake cannot enroll or prepare a Cloud handoff", () => {
  assert.deepEqual(
    intakeDecision({
      eventName: "issues",
      action: "opened",
      issueLabels: ["quick-capture"],
      trustedActor: false,
    }),
    { enroll: false, prepareHandoff: false, reason: "untrusted-actor" },
  );
  assert.equal(
    intakeDecision({
      eventName: "issues",
      action: "labeled",
      label: "agent-task",
      trustedActor: true,
    }).prepareHandoff,
    true,
  );
});

test("commands ignore bots and unrelated pull requests", () => {
  assert.equal(
    commandDecision({
      body: "/agent-run",
      trustedActor: true,
      actorType: "Bot",
      subjectKind: "issue",
      issueLabels: ["agent-task"],
    }).prepareHandoff,
    false,
  );
  assert.equal(
    commandDecision({
      body: "/agent-run",
      trustedActor: true,
      subjectKind: "pull_request",
      pullRequestBody: "Ordinary pull request",
    }).prepareHandoff,
    false,
  );
});

test("Project transitions reject skips and make retries idempotent", () => {
  assert.deepEqual(transitionDecision("Inbox", "Triaging"), {
    allowed: true,
    idempotent: false,
    reason: "valid-transition",
  });
  assert.deepEqual(transitionDecision("Triaging", "Triaging"), {
    allowed: true,
    idempotent: true,
    reason: "already-current",
  });
  assert.equal(transitionDecision("Inbox", "PR review").allowed, false);
  assert.equal(transitionDecision("mystery", "Done").allowed, false);
  assert.equal(transitionDecision("Blocked", "Human review").allowed, true);
});

test("prompt sanitization removes hidden HTML and enforces a bound", () => {
  assert.equal(sanitizePromptText("visible<!-- hidden -->\0 text", 12), "visible text");
  assert.equal(sanitizePromptText("123456", 4), "1234");
});

test("validates the exact triage contract", () => {
  assert.deepEqual(validateTriage(smallTriage), { valid: true, errors: [] });
  assert.equal(validateTriage({ ...smallTriage, track: "tiny" }).valid, false);
  assert.equal(validateTriage({ ...smallTriage, unexpected: true }).valid, false);
  assert.equal(
    validateTriage({ ...smallTriage, track: "needs_input", questions: [] }).valid,
    false,
  );
  assert.equal(
    validateTriage({ ...smallTriage, track: "exec_plan", memory: "decision" }).valid,
    false,
  );
  assert.equal(validateTriage({ ...smallTriage, summary: "x".repeat(241) }).valid, false);
  assert.equal(
    validateTriage({ ...smallTriage, scope: Array.from({ length: 21 }, () => "item") }).valid,
    false,
  );
  assert.equal(
    validateTriage({ ...smallTriage, validation: ["x".repeat(1_001)] }).valid,
    false,
  );
});

test("renders one versioned Small contract without hidden comment injection", () => {
  const rendered = renderAgentContract(
    { ...smallTriage, scope: ["Keep this<!-- forge --> bounded."] },
    { issueNumber: 42, revision: 3 },
  );
  assert.match(rendered, /lumilio-agent-contract:v1 issue=42 revision=3/);
  assert.match(rendered, /## Validation/);
  assert.doesNotMatch(rendered, /forge/);
  assert.equal((rendered.match(/lumilio-agent-contract:v1/g) ?? []).length, 1);
});

test("plan-only guard accepts one valid new regular plan", () => {
  assert.deepEqual(
    validatePlanPatch(
      [
        {
          path: "site/docs/internal/exec-plans/active/issue-42-repair-task.md",
          status: "added",
          type: "file",
          content: validPlan,
        },
      ],
      42,
    ),
    { valid: true, errors: [] },
  );
});

test("plan-only guard rejects one allowed file plus one forbidden file", () => {
  const result = validatePlanPatch(
    [
      {
        path: "site/docs/internal/exec-plans/active/issue-42-repair-task.md",
        status: "added",
        type: "file",
        content: validPlan,
      },
      { path: "server/cmd/main.go", status: "modified", type: "file", content: "" },
    ],
    42,
  );
  assert.equal(result.valid, false);
  assert.match(result.errors.join("\n"), /exactly one/);
});

test("plan-only guard rejects traversal, symlinks, wrong issue, and missing sections", () => {
  for (const change of [
    {
      path: "site/docs/internal/exec-plans/active/../issue-42-escape.md",
      status: "added",
      type: "file",
      content: validPlan,
    },
    {
      path: "site/docs/internal/exec-plans/active/issue-42-link.md",
      status: "added",
      type: "symlink",
      content: validPlan,
    },
    {
      path: "site/docs/internal/exec-plans/active/issue-99-wrong.md",
      status: "added",
      type: "file",
      content: validPlan,
    },
    {
      path: "site/docs/internal/exec-plans/active/issue-42-short.md",
      status: "added",
      type: "file",
      content: "# Too short\n",
    },
  ]) {
    assert.equal(validatePlanPatch([change], 42).valid, false);
  }
});

test("implementation gate rejects stale revisions and duplicate active runs", () => {
  const base = {
    issueOpen: true,
    status: "Human review",
    track: "small",
    currentRevision: 2,
    approvedRevision: 2,
    activeRun: false,
  };
  assert.deepEqual(implementationGate(base), { allowed: true, reason: "approved" });
  assert.equal(implementationGate({ ...base, currentRevision: 3 }).reason, "stale-revision");
  assert.equal(implementationGate({ ...base, activeRun: true }).reason, "active-run");
  assert.equal(
    implementationGate({ ...base, track: "exec_plan", status: "Ready" }).reason,
    "plan-not-merged",
  );
});

test("task markers are strict and plan markers cannot claim final completion", () => {
  const marker = renderTaskMarker({
    issueNumber: 42,
    role: "implementation",
    revision: 2,
    completion: "final",
  });
  assert.deepEqual(parseTaskMarker(marker), {
    issueNumber: 42,
    role: "implementation",
    revision: 2,
    completion: "final",
  });
  assert.throws(() =>
    renderTaskMarker({ issueNumber: 42, role: "plan", revision: 1, completion: "final" }),
  );
  assert.equal(parseTaskMarker(`${marker}\n${marker}`), null);
});

test("closer accepts only a final implementation merged to dev", () => {
  const finalBody = renderTaskMarker({
    issueNumber: 42,
    role: "implementation",
    revision: 2,
    completion: "final",
  });
  const base = {
    merged: true,
    baseBranch: "dev",
    pullRequestBody: finalBody,
    issueOpen: true,
    issueLabels: ["agent-task"],
  };
  assert.deepEqual(closeDecision(base), {
    close: true,
    reason: "final-implementation",
    issueNumber: 42,
  });
  assert.equal(closeDecision({ ...base, baseBranch: "main" }).close, false);
  assert.equal(closeDecision({ ...base, merged: false }).close, false);
  assert.equal(closeDecision({ ...base, issueOpen: false }).reason, "already-closed");

  const planBody = renderTaskMarker({
    issueNumber: 42,
    role: "plan",
    revision: 2,
    completion: "intermediate",
  });
  assert.equal(closeDecision({ ...base, pullRequestBody: planBody }).reason, "not-implementation");

  const intermediateBody = renderTaskMarker({
    issueNumber: 42,
    role: "implementation",
    revision: 2,
    completion: "intermediate",
  });
  assert.equal(closeDecision({ ...base, pullRequestBody: intermediateBody }).reason, "not-final");
});

test("manual close and reopen map to deterministic Project states", () => {
  assert.equal(statusForIssueAction("closed"), "Done");
  assert.equal(statusForIssueAction("reopened"), "Human review");
  assert.equal(statusForIssueAction("edited"), null);
});
