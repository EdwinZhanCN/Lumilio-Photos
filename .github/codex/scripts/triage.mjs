import { renderAgentContract, sanitizePromptText, validateTriage } from "./policy.mjs";

export const CONTRACT_MARKER = "<!-- lumilio-agent-contract:v1 ";
export const CLOUD_HANDOFF_MARKER = "<!-- lumilio-agent-cloud-handoff:v1 ";
export const NEEDS_INPUT_MARKER = "<!-- lumilio-agent-needs-input:v1 ";
export const PLAN_REQUEST_MARKER = "<!-- lumilio-agent-plan-request:v1 ";
export const DIAGNOSTIC_MARKER = "<!-- lumilio-agent-diagnostic:v1 ";
export const TRIAGE_MARKERS = Object.freeze([
  CONTRACT_MARKER,
  NEEDS_INPUT_MARKER,
  PLAN_REQUEST_MARKER,
]);

const AREA_PROJECT_VALUES = Object.freeze({
  server: "Server",
  web: "Web",
  desktop: "Desktop",
  deploy: "Deploy",
  docs: "Docs",
  "cross-cutting": "Cross-cutting",
  unknown: "Unknown",
});

export function parseTriageResponse(text) {
  if (typeof text !== "string" || text.length > 100_000) {
    throw new Error("triage response must be bounded JSON text");
  }
  let value;
  try {
    value = JSON.parse(text.trim());
  } catch {
    throw new Error("triage response is not a JSON object");
  }
  const result = validateTriage(value);
  if (!result.valid) throw new Error(`invalid triage response: ${result.errors.join("; ")}`);
  return value;
}

export function renderTriagePrompt(template, issue, openIssues, reviewFeedback = "") {
  const context = {
    issue: {
      number: issue.number,
      url: issue.html_url,
      author: sanitizePromptText(issue.user?.login, 100),
      title: sanitizePromptText(issue.title, 500),
      body: sanitizePromptText(issue.body, 12_000),
      labels: (issue.labels ?? []).map((label) => sanitizePromptText(label.name ?? label, 80)),
    },
    open_issue_index: openIssues,
    review_feedback: sanitizePromptText(reviewFeedback, 8_000),
  };
  return `${template.trim()}\n\n--- BEGIN UNTRUSTED ISSUE CONTEXT ---\n\n${JSON.stringify(context, null, 2)}\n\n--- END UNTRUSTED ISSUE CONTEXT ---\n`;
}

export function revisionForMode(comments, markerPrefixes, mode) {
  const prefixes = Array.isArray(markerPrefixes) ? markerPrefixes : [markerPrefixes];
  const managed = comments.filter(
    (comment) =>
      comment.user?.login === "github-actions[bot]" &&
      typeof comment.body === "string" &&
      prefixes.some((markerPrefix) => comment.body.startsWith(markerPrefix)),
  );
  const revisions = managed
    .map((comment) => comment.body.match(/\brevision=(\d+)\b/))
    .filter(Boolean)
    .map((match) => Number(match[1]));
  const current = revisions.length === 0 ? 0 : Math.max(...revisions);
  if (mode === "revise") return current + 1;
  if (mode === "submit") return current + 1 || 1;
  if (mode === "initial" || mode === "retry") return current || 1;
  throw new Error(`unknown triage publish mode: ${mode}`);
}

export function renderCloudHandoff(
  prompt,
  { issueNumber, repository, baseBranch, mode, runUrl },
) {
  if (typeof prompt !== "string" || prompt.trim() === "" || prompt.length > 40_000) {
    throw new Error("Cloud handoff prompt must be bounded non-empty text");
  }
  const safeRepository = sanitizePromptText(repository, 200);
  const safeBaseBranch = sanitizePromptText(baseBranch, 200);
  const safeMode = sanitizePromptText(mode, 40);
  const indentedPrompt = prompt
    .trim()
    .split("\n")
    .map((line) => `    ${line}`)
    .join("\n");
  return [
    `<!-- lumilio-agent-cloud-handoff:v1 issue=${issueNumber} stage=triage -->`,
    `**Codex Cloud handoff · ${safeMode}**`,
    "",
    "This GitHub Action did not call a model and does not use an OpenAI API key.",
    "Start the preflight manually so it uses the Codex allowance in your ChatGPT subscription:",
    "",
    "1. Open [Codex Cloud](https://chatgpt.com/codex).",
    `2. Select \`${safeRepository}\` and start from \`${safeBaseBranch}\`.`,
    "3. Start a cloud task with the prompt below. Do not ask it to implement yet.",
    "4. Paste its JSON-only final response back into this Issue as a trusted comment",
    "   whose first line is `/agent-submit`.",
    "",
    "Copy this prompt:",
    "",
    indentedPrompt,
    "",
    "The `/agent-submit` Action validates the JSON before it can update the managed",
    "contract or Project fields.",
    "",
    `[Handoff preparation run](${runUrl})`,
  ].join("\n");
}

export function triageProjectUpdates(triage, runUrl) {
  const updates = {
    Area: AREA_PROJECT_VALUES[triage.area],
    Priority: triage.priority,
    "Agent run": runUrl,
  };
  if (triage.track === "small") {
    updates.Track = "Small";
    updates.Status = "Human review";
  } else if (triage.track === "exec_plan") {
    updates.Track = "Exec Plan";
    updates.Status = "Human review";
  } else {
    updates.Status = "Needs input";
  }
  return updates;
}

export function renderNeedsInput(triage, { issueNumber, revision, runUrl }) {
  return [
    `<!-- lumilio-agent-needs-input:v1 issue=${issueNumber} revision=${revision} -->`,
    `**Agent preflight · needs input · revision ${revision}**`,
    "",
    sanitizePromptText(triage.summary, 1_000),
    "",
    "## Questions",
    ...triage.questions.map((question) => `- ${sanitizePromptText(question, 1_000)}`),
    "",
    `[Orchestration run](${runUrl})`,
  ].join("\n");
}

export function renderPlanRequest(triage, { issueNumber, revision, runUrl }) {
  return [
    `<!-- lumilio-agent-plan-request:v1 issue=${issueNumber} revision=${revision} -->`,
    `**Exec Plan requested · revision ${revision}**`,
    "",
    sanitizePromptText(triage.summary, 1_000),
    "",
    "## Proposed scope",
    ...triage.scope.map((item) => `- ${sanitizePromptText(item, 1_000)}`),
    "",
    "## Validation boundaries",
    ...triage.validation.map((item) => `- ${sanitizePromptText(item, 1_000)}`),
    "",
    "The plan-only Draft PR is the next gated transition; this comment is not the execution contract.",
    "",
    `[Orchestration run](${runUrl})`,
  ].join("\n");
}

export function renderSmallContract(triage, { issueNumber, revision, runUrl }) {
  return `${renderAgentContract(triage, { issueNumber, revision })}\n\n[Orchestration run](${runUrl})`;
}

export function renderDiagnostic({ issueNumber, stage, message, runUrl }) {
  return [
    `<!-- lumilio-agent-diagnostic:v1 issue=${issueNumber} stage=${stage} -->`,
    `**Agent loop blocked during ${sanitizePromptText(stage, 80)}.**`,
    "",
    sanitizePromptText(message, 800),
    "",
    "No automatic retry will run. Correct the configuration or input, then use `/agent-retry` when that command is enabled.",
    "",
    `[Orchestration run](${runUrl})`,
  ].join("\n");
}
