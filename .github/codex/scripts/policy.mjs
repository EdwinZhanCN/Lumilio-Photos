const COMMANDS = new Set([
  "/agent-revise",
  "/agent-run",
  "/agent-retry",
  "/agent-submit",
]);

export const PROJECT_STATUSES = Object.freeze([
  "Inbox",
  "Triaging",
  "Needs input",
  "Human review",
  "Ready",
  "Running",
  "PR review",
  "Blocked",
  "Done",
]);

export const TRIAGE_TRACKS = Object.freeze(["small", "exec_plan", "needs_input"]);
export const TRIAGE_AREAS = Object.freeze([
  "server",
  "web",
  "desktop",
  "deploy",
  "docs",
  "cross-cutting",
  "unknown",
]);

const TRUSTED_PERMISSIONS = new Set(["write", "maintain", "admin"]);
const TRUSTED_ASSOCIATIONS = new Set(["OWNER", "MEMBER", "COLLABORATOR"]);
const TRIAGE_PRIORITIES = new Set(["P0", "P1", "P2", "P3"]);
const TRIAGE_MEMORIES = new Set([
  "none",
  "decision",
  "postmortem",
  "active_plan",
]);
const TASK_ROLES = new Set(["plan", "implementation"]);
const COMPLETIONS = new Set(["intermediate", "final"]);

const STATUS_TRANSITIONS = Object.freeze({
  Inbox: new Set(["Triaging", "Blocked", "Done"]),
  Triaging: new Set(["Needs input", "Human review", "Blocked", "Done"]),
  "Needs input": new Set(["Triaging", "Blocked", "Done"]),
  "Human review": new Set(["Triaging", "Ready", "Running", "Blocked", "Done"]),
  Ready: new Set(["Running", "Blocked", "Done"]),
  Running: new Set(["PR review", "Blocked", "Done"]),
  "PR review": new Set(["Running", "Blocked", "Done"]),
  Blocked: new Set(["Triaging", "Needs input", "Human review", "Running", "Done"]),
  Done: new Set(["Human review"]),
});

const REQUIRED_TRIAGE_KEYS = Object.freeze([
  "track",
  "summary",
  "area",
  "priority",
  "scope",
  "non_goals",
  "todos",
  "validation",
  "memory",
  "questions",
  "related",
]);

const PLAN_PREFIX = "site/docs/internal/exec-plans/active/";
const REQUIRED_PLAN_PATTERNS = Object.freeze([
  /^#\s+\S/m,
  /^Status:\s+\S/m,
  /^Goal:\s+\S/m,
  /^## Non-goals\s*$/m,
  /^## Execution phases\s*$/m,
  /^### Phase 0\s+—\s+Lock the failures\s*$/m,
  /^## Validation boundaries\s*$/m,
]);

export function parseAgentCommand(body) {
  if (typeof body !== "string") return null;

  const lines = body.replaceAll("\r\n", "\n").split("\n");
  const commandIndex = lines.findIndex((line) => line.trim() !== "");
  if (commandIndex === -1) return null;

  const command = lines[commandIndex].trim();
  if (!COMMANDS.has(command)) return null;

  return {
    command: command.slice("/agent-".length),
    feedback: lines.slice(commandIndex + 1).join("\n").trim(),
  };
}

export function isTrustedActor({ permission, association, actorType = "User" }) {
  return (
    actorType !== "Bot" &&
    TRUSTED_PERMISSIONS.has(permission) &&
    TRUSTED_ASSOCIATIONS.has(association)
  );
}

export function intakeDecision({
  eventName,
  action,
  label,
  issueLabels = [],
  trustedActor,
}) {
  if (!trustedActor) {
    return { enroll: false, prepareHandoff: false, reason: "untrusted-actor" };
  }

  if (eventName === "workflow_dispatch") {
    return { enroll: true, prepareHandoff: true, reason: "manual-recovery" };
  }

  if (eventName !== "issues") {
    return { enroll: false, prepareHandoff: false, reason: "unsupported-event" };
  }

  if (
    (action === "opened" || action === "reopened") &&
    issueLabels.includes("quick-capture")
  ) {
    return { enroll: true, prepareHandoff: true, reason: "trusted-quick-capture" };
  }

  if (action === "labeled" && label === "agent-task") {
    return { enroll: true, prepareHandoff: true, reason: "trusted-enrollment" };
  }

  return { enroll: false, prepareHandoff: false, reason: "not-enrolled" };
}

export function commandDecision({
  body,
  trustedActor,
  actorType = "User",
  subjectKind,
  issueLabels = [],
  pullRequestBody = "",
}) {
  const parsed = parseAgentCommand(body);
  if (!parsed) return { prepareHandoff: false, reason: "not-a-command" };
  if (!trustedActor || actorType === "Bot") {
    return { prepareHandoff: false, reason: "untrusted-actor" };
  }

  if (subjectKind === "issue" && issueLabels.includes("agent-task")) {
    return { prepareHandoff: true, reason: "managed-issue", ...parsed };
  }

  if (subjectKind === "pull_request" && parseTaskMarker(pullRequestBody)) {
    return { prepareHandoff: true, reason: "managed-pull-request", ...parsed };
  }

  return { prepareHandoff: false, reason: "unmanaged-subject" };
}

export function transitionDecision(current, next) {
  if (!PROJECT_STATUSES.includes(current) || !PROJECT_STATUSES.includes(next)) {
    return { allowed: false, idempotent: false, reason: "unknown-status" };
  }
  if (current === next) {
    return { allowed: true, idempotent: true, reason: "already-current" };
  }
  if (STATUS_TRANSITIONS[current].has(next)) {
    return { allowed: true, idempotent: false, reason: "valid-transition" };
  }
  return { allowed: false, idempotent: false, reason: "invalid-transition" };
}

export function sanitizePromptText(value, maxLength = 12_000) {
  if (typeof value !== "string") return "";
  return value
    .replaceAll(/<!--[\s\S]*?-->/g, "")
    .replaceAll("\0", "")
    .slice(0, maxLength)
    .trim();
}

export function validateTriage(value) {
  const errors = [];
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { valid: false, errors: ["triage output must be an object"] };
  }

  const keys = Object.keys(value).sort();
  const expected = [...REQUIRED_TRIAGE_KEYS].sort();
  if (keys.length !== expected.length || keys.some((key, index) => key !== expected[index])) {
    errors.push("triage output must contain exactly the versioned contract keys");
  }

  if (!TRIAGE_TRACKS.includes(value.track)) errors.push("track is invalid");
  if (
    typeof value.summary !== "string" ||
    value.summary.trim() === "" ||
    value.summary.length > 240
  ) {
    errors.push("summary must be a non-empty string no longer than 240 characters");
  }
  if (!TRIAGE_AREAS.includes(value.area)) errors.push("area is invalid");
  if (!TRIAGE_PRIORITIES.has(value.priority)) errors.push("priority is invalid");
  if (!TRIAGE_MEMORIES.has(value.memory)) errors.push("memory is invalid");

  for (const key of ["scope", "non_goals", "todos", "validation", "questions", "related"]) {
    if (!isBoundedStringArray(value[key])) {
      errors.push(`${key} must contain at most 20 non-empty strings of at most 1000 characters`);
    }
  }

  if (value.track === "needs_input") {
    if (!Array.isArray(value.questions) || value.questions.length === 0) {
      errors.push("needs_input requires at least one question");
    }
  } else {
    for (const key of ["scope", "todos", "validation"]) {
      if (!Array.isArray(value[key]) || value[key].length === 0) {
        errors.push(`${value.track} requires at least one ${key} item`);
      }
    }
  }

  if (value.track === "exec_plan" && value.memory !== "active_plan") {
    errors.push("exec_plan requires active_plan memory");
  }

  return { valid: errors.length === 0, errors };
}

export function renderAgentContract(triage, { issueNumber, revision }) {
  const validation = validateTriage(triage);
  if (!validation.valid || triage.track !== "small") {
    throw new Error(`cannot render Small contract: ${validation.errors.join("; ")}`);
  }
  if (!Number.isSafeInteger(issueNumber) || issueNumber <= 0) {
    throw new Error("issueNumber must be a positive integer");
  }
  if (!Number.isSafeInteger(revision) || revision <= 0) {
    throw new Error("revision must be a positive integer");
  }

  return [
    `<!-- lumilio-agent-contract:v1 issue=${issueNumber} revision=${revision} -->`,
    `**Agent Contract · revision ${revision}**`,
    "",
    "## Scope",
    renderList(triage.scope),
    "",
    "## Non-goals",
    renderList(triage.non_goals),
    "",
    "## Todos",
    renderChecklist(triage.todos),
    "",
    "## Validation",
    renderChecklist(triage.validation),
    "",
    "## Documentation / Memory",
    `- ${sanitizePromptText(triage.memory, 80)}`,
    "",
    "## Open questions",
    renderList(triage.questions),
  ].join("\n");
}

export function validatePlanPatch(changes, issueNumber) {
  const errors = [];
  if (!Array.isArray(changes) || changes.length !== 1) {
    errors.push("plan patch must contain exactly one changed path");
    return { valid: false, errors };
  }

  const change = changes[0];
  const path = typeof change.path === "string" ? change.path : "";
  const escaped =
    path.includes("\\") ||
    path.split("/").includes("..") ||
    !path.startsWith(PLAN_PREFIX);
  const expectedPath = new RegExp(
    `^${escapeRegExp(PLAN_PREFIX)}issue-${issueNumber}-[a-z0-9]+(?:-[a-z0-9]+)*\\.md$`,
  );

  if (escaped || !expectedPath.test(path)) errors.push("plan path is outside the issue allowlist");
  if (change.status !== "added") errors.push("plan file must be newly added");
  if (change.type !== "file") errors.push("plan path must be a regular file");
  if (typeof change.content !== "string") {
    errors.push("plan content must be text");
  } else {
    for (const pattern of REQUIRED_PLAN_PATTERNS) {
      if (!pattern.test(change.content)) {
        errors.push(`plan is missing required structure: ${pattern.source}`);
      }
    }
    if (!new RegExp(`Issue\\s+#${issueNumber}\\b`, "i").test(change.content)) {
      errors.push("plan Status must identify the linked Issue");
    }
  }

  return { valid: errors.length === 0, errors };
}

export function implementationGate({
  issueOpen,
  status,
  track,
  currentRevision,
  approvedRevision,
  activeRun,
  planMerged = false,
}) {
  if (!issueOpen) return { allowed: false, reason: "issue-closed" };
  if (status === "Needs input") return { allowed: false, reason: "needs-input" };
  if (activeRun) return { allowed: false, reason: "active-run" };
  if (
    !Number.isSafeInteger(currentRevision) ||
    !Number.isSafeInteger(approvedRevision) ||
    currentRevision !== approvedRevision
  ) {
    return { allowed: false, reason: "stale-revision" };
  }
  if (track === "exec_plan" && !planMerged) {
    return { allowed: false, reason: "plan-not-merged" };
  }
  if (track !== "small" && track !== "exec_plan") {
    return { allowed: false, reason: "invalid-track" };
  }
  if (status !== "Human review" && status !== "Ready" && status !== "PR review") {
    return { allowed: false, reason: "invalid-status" };
  }
  return { allowed: true, reason: "approved" };
}

export function renderTaskMarker({ issueNumber, role, revision, completion }) {
  if (!Number.isSafeInteger(issueNumber) || issueNumber <= 0) {
    throw new Error("issueNumber must be a positive integer");
  }
  if (!TASK_ROLES.has(role)) throw new Error("role is invalid");
  if (!Number.isSafeInteger(revision) || revision <= 0) {
    throw new Error("revision must be a positive integer");
  }
  if (!COMPLETIONS.has(completion)) throw new Error("completion is invalid");
  if (role === "plan" && completion !== "intermediate") {
    throw new Error("plan markers are always intermediate");
  }
  return `<!-- lumilio-agent-task:v1 issue=${issueNumber} role=${role} revision=${revision} completion=${completion} -->`;
}

export function parseTaskMarker(body) {
  if (typeof body !== "string") return null;
  const pattern =
    /<!-- lumilio-agent-task:v1 issue=(\d+) role=(plan|implementation) revision=(\d+) completion=(intermediate|final) -->/g;
  const matches = [...body.matchAll(pattern)];
  if (matches.length !== 1) return null;
  const [, issue, role, revision, completion] = matches[0];
  const marker = {
    issueNumber: Number(issue),
    role,
    revision: Number(revision),
    completion,
  };
  if (role === "plan" && completion !== "intermediate") return null;
  return marker;
}

export function closeDecision({
  merged,
  baseBranch,
  expectedBaseBranch = "dev",
  pullRequestBody,
  issueOpen,
  issueLabels = [],
}) {
  if (!merged) return { close: false, reason: "not-merged" };
  if (baseBranch !== expectedBaseBranch) return { close: false, reason: "wrong-base" };

  const marker = parseTaskMarker(pullRequestBody);
  if (!marker) return { close: false, reason: "missing-marker" };
  if (marker.role !== "implementation") {
    return { close: false, reason: "not-implementation" };
  }
  if (marker.completion !== "final") return { close: false, reason: "not-final" };
  if (!issueLabels.includes("agent-task")) {
    return { close: false, reason: "unmanaged-issue", issueNumber: marker.issueNumber };
  }
  if (!issueOpen) {
    return { close: false, reason: "already-closed", issueNumber: marker.issueNumber };
  }
  return { close: true, reason: "final-implementation", issueNumber: marker.issueNumber };
}

export function statusForIssueAction(action) {
  if (action === "closed") return "Done";
  if (action === "reopened") return "Human review";
  return null;
}

function isBoundedStringArray(value) {
  return (
    Array.isArray(value) &&
    value.length <= 20 &&
    value.every(
      (item) =>
        typeof item === "string" &&
        item.trim() !== "" &&
        item.length <= 1_000,
    )
  );
}

function renderList(items) {
  if (items.length === 0) return "- None.";
  return items.map((item) => `- ${sanitizePromptText(item, 1_000)}`).join("\n");
}

function renderChecklist(items) {
  if (items.length === 0) return "- [ ] None.";
  return items.map((item) => `- [ ] ${sanitizePromptText(item, 1_000)}`).join("\n");
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
