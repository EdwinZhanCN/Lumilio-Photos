import { appendFileSync, readFileSync } from "node:fs";

import {
  authorizeCommand,
  authorizeIntake,
  deleteIssueComment,
  ensureIssueLabel,
  githubRest,
  listIssueComments,
  listOpenIssueIndex,
  upsertManagedComment,
} from "./github.mjs";
import { parseAgentCommand } from "./policy.mjs";
import { syncProjectItem } from "./project.mjs";
import {
  CLOUD_HANDOFF_MARKER,
  CONTRACT_MARKER,
  DIAGNOSTIC_MARKER,
  NEEDS_INPUT_MARKER,
  PLAN_REQUEST_MARKER,
  TRIAGE_MARKERS,
  parseTriageResponse,
  renderCloudHandoff,
  renderDiagnostic,
  renderNeedsInput,
  renderPlanRequest,
  renderSmallContract,
  renderTriagePrompt,
  revisionForMode,
  triageProjectUpdates,
} from "./triage.mjs";

const command = process.argv[2];

try {
  if (command === "authorize") await authorize();
  else if (command === "authorize-command") await authorizeCommandEvent();
  else if (command === "enroll") await enroll();
  else if (command === "prepare-cloud-handoff") await prepareCloudHandoff();
  else if (command === "publish-comment-triage") await publishCommentTriage();
  else if (command === "resume") await resume();
  else if (command === "block") await block();
  else {
    throw new Error(
      "usage: agent-loop.mjs authorize|authorize-command|enroll|prepare-cloud-handoff|publish-comment-triage|resume|block",
    );
  }
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}

async function authorize() {
  const event = JSON.parse(readFileSync(required("GITHUB_EVENT_PATH"), "utf8"));
  const result = await authorizeIntake({
    eventName: required("GITHUB_EVENT_NAME"),
    event,
    repository: required("GITHUB_REPOSITORY"),
    actorLogin: required("GITHUB_ACTOR"),
    manualIssueNumber: process.env.ISSUE_NUMBER,
    token: required("GITHUB_TOKEN"),
  });
  writeOutput("authorized", String(result.authorized));
  writeOutput("reason", result.reason);
  writeOutput("issue_number", String(result.issueNumber));
  writeOutput("content_id", result.contentId);
}

async function authorizeCommandEvent() {
  const event = JSON.parse(readFileSync(required("GITHUB_EVENT_PATH"), "utf8"));
  const result = await authorizeCommand({
    eventName: required("GITHUB_EVENT_NAME"),
    event,
    repository: required("GITHUB_REPOSITORY"),
    actorLogin: required("GITHUB_ACTOR"),
    manualIssueNumber: process.env.ISSUE_NUMBER,
    manualCommand: process.env.AGENT_COMMAND,
    token: required("GITHUB_TOKEN"),
  });
  writeOutput("authorized", String(result.authorized));
  writeOutput("reason", result.reason);
  if (result.issueNumber) writeOutput("issue_number", String(result.issueNumber));
  if (result.contentId) writeOutput("content_id", result.contentId);
  if (result.mode) writeOutput("mode", result.mode);
  if (result.commentId) writeOutput("comment_id", String(result.commentId));
  writeOutput("resume", result.resume ?? "");
}

async function enroll() {
  const issueNumber = positiveInteger(required("ISSUE_NUMBER"), "ISSUE_NUMBER");
  const repository = required("GITHUB_REPOSITORY");
  const issueToken = required("GITHUB_TOKEN");
  const runUrl = required("AGENT_RUN_URL");
  try {
    await ensureIssueLabel({
      token: issueToken,
      repository,
      issueNumber,
      label: "agent-task",
    });
    await syncProjectItem({
      token: required("AGENT_PROJECT_TOKEN"),
      owner: required("AGENT_PROJECT_OWNER"),
      number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
      contentId: required("CONTENT_ID"),
      updates: { Status: "Triaging", "Agent run": runUrl },
      bootstrapStatus: true,
    });
  } catch (error) {
    await bestEffortDiagnostic({
      issueToken,
      repository,
      issueNumber,
      stage: "enrollment",
      error,
      runUrl,
    });
    throw error;
  }
}

async function prepareCloudHandoff() {
  const issueNumber = positiveInteger(required("ISSUE_NUMBER"), "ISSUE_NUMBER");
  const repository = required("GITHUB_REPOSITORY");
  const issueToken = required("GITHUB_TOKEN");
  const runUrl = required("AGENT_RUN_URL");
  const mode = process.env.HANDOFF_MODE || "initial";
  const issue = await githubRest({ token: issueToken, repository, path: `/issues/${issueNumber}` });
  if (issue.pull_request) throw new Error("triage target must be an Issue");
  const openIssues = await listOpenIssueIndex({
    token: issueToken,
    repository,
    excludeIssue: issueNumber,
  });
  let reviewFeedback = "";
  const feedbackCommentId = process.env.FEEDBACK_COMMENT_ID;
  if (mode === "revise" && feedbackCommentId) {
    const commentId = positiveInteger(feedbackCommentId, "FEEDBACK_COMMENT_ID");
    const comment = await githubRest({ token: issueToken, repository, path: `/issues/comments/${commentId}` });
    const parsed = parseAgentCommand(comment.body);
    if (!parsed || parsed.command !== "revise") {
      throw new Error("trusted revision comment no longer contains /agent-revise");
    }
    reviewFeedback = parsed.feedback;
  }
  const template = readFileSync(required("TRIAGE_PROMPT_TEMPLATE"), "utf8");
  const prompt = renderTriagePrompt(template, issue, openIssues, reviewFeedback);
  try {
    await upsertManagedComment({
      token: issueToken,
      repository,
      issueNumber,
      markerPrefix: CLOUD_HANDOFF_MARKER,
      body: renderCloudHandoff(prompt, {
        issueNumber,
        repository,
        baseBranch: process.env.AGENT_BASE_BRANCH || "dev",
        mode,
        runUrl,
      }),
    });
    await syncProjectItem({
      token: required("AGENT_PROJECT_TOKEN"),
      owner: required("AGENT_PROJECT_OWNER"),
      number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
      contentId: required("CONTENT_ID"),
      updates: { Status: "Triaging", "Agent run": runUrl },
    });
  } catch (error) {
    await bestEffortDiagnostic({
      issueToken,
      repository,
      issueNumber,
      stage: "preparing the Codex Cloud handoff",
      error,
      runUrl,
    });
    throw error;
  }
}

async function publishCommentTriage() {
  const repository = required("GITHUB_REPOSITORY");
  const issueToken = required("GITHUB_TOKEN");
  const commentId = positiveInteger(required("FEEDBACK_COMMENT_ID"), "FEEDBACK_COMMENT_ID");
  const comment = await githubRest({
    token: issueToken,
    repository,
    path: `/issues/comments/${commentId}`,
  });
  const parsed = parseAgentCommand(comment.body);
  if (!parsed || parsed.command !== "submit" || parsed.feedback === "") {
    throw new Error("trusted submission comment no longer contains /agent-submit followed by JSON");
  }
  await publishTriage(parsed.feedback);
}

async function publishTriage(text) {
  const issueNumber = positiveInteger(required("ISSUE_NUMBER"), "ISSUE_NUMBER");
  const repository = required("GITHUB_REPOSITORY");
  const issueToken = required("GITHUB_TOKEN");
  const projectToken = process.env.AGENT_PROJECT_TOKEN ?? "";
  const runUrl = required("AGENT_RUN_URL");
  const mode = process.env.TRIAGE_MODE || "initial";
  let triage;
  try {
    triage = parseTriageResponse(text);
    const comments = await listIssueComments({ token: issueToken, repository, issueNumber });
    const marker =
      triage.track === "small"
        ? CONTRACT_MARKER
        : triage.track === "exec_plan"
          ? PLAN_REQUEST_MARKER
          : NEEDS_INPUT_MARKER;
    const revision = revisionForMode(comments, TRIAGE_MARKERS, mode);
    const body =
      triage.track === "small"
        ? renderSmallContract(triage, { issueNumber, revision, runUrl })
        : triage.track === "exec_plan"
          ? renderPlanRequest(triage, { issueNumber, revision, runUrl })
          : renderNeedsInput(triage, { issueNumber, revision, runUrl });

    await syncProjectItem({
      token: projectToken,
      owner: required("AGENT_PROJECT_OWNER"),
      number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
      contentId: required("CONTENT_ID"),
      updates: Object.fromEntries(
        Object.entries(triageProjectUpdates(triage, runUrl)).filter(([key]) => key !== "Status"),
      ),
    });
    await upsertManagedComment({
      token: issueToken,
      repository,
      issueNumber,
      markerPrefix: marker,
      body,
    });
    for (const comment of comments) {
      if (
        comment.user?.login === "github-actions[bot]" &&
        TRIAGE_MARKERS.some(
          (markerPrefix) => markerPrefix !== marker && comment.body?.startsWith(markerPrefix),
        )
      ) {
        await deleteIssueComment({ token: issueToken, repository, commentId: comment.id });
      }
      if (
        comment.user?.login === "github-actions[bot]" &&
        comment.body?.startsWith(CLOUD_HANDOFF_MARKER)
      ) {
        await deleteIssueComment({ token: issueToken, repository, commentId: comment.id });
      }
    }
    await syncProjectItem({
      token: projectToken,
      owner: required("AGENT_PROJECT_OWNER"),
      number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
      contentId: required("CONTENT_ID"),
      updates: {
        Status: triageProjectUpdates(triage, runUrl).Status,
        "Agent run": runUrl,
      },
    });
  } catch (error) {
    try {
      await syncProjectItem({
        token: projectToken,
        owner: required("AGENT_PROJECT_OWNER"),
        number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
        contentId: required("CONTENT_ID"),
        updates: { Status: "Blocked", "Agent run": runUrl },
      });
    } catch (projectError) {
      process.stderr.write(`Could not set Project Blocked: ${messageOf(projectError)}\n`);
    }
    await bestEffortDiagnostic({
      issueToken,
      repository,
      issueNumber,
      stage: "triage publishing",
      error,
      runUrl,
    });
    throw error;
  }
}

async function block() {
  const issueNumber = positiveInteger(required("ISSUE_NUMBER"), "ISSUE_NUMBER");
  const repository = required("GITHUB_REPOSITORY");
  const issueToken = required("GITHUB_TOKEN");
  const projectToken = required("AGENT_PROJECT_TOKEN");
  const runUrl = required("AGENT_RUN_URL");
  const stage = process.env.BLOCK_STAGE || "orchestration";
  const message = process.env.BLOCK_MESSAGE || "The orchestration job did not complete successfully.";
  try {
    await syncProjectItem({
      token: projectToken,
      owner: required("AGENT_PROJECT_OWNER"),
      number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
      contentId: required("CONTENT_ID"),
      updates: { Status: "Blocked", "Agent run": runUrl },
    });
  } finally {
    await bestEffortDiagnostic({
      issueToken,
      repository,
      issueNumber,
      stage,
      error: new Error(message),
      runUrl,
    });
  }
}

async function resume() {
  const issueNumber = positiveInteger(required("ISSUE_NUMBER"), "ISSUE_NUMBER");
  const repository = required("GITHUB_REPOSITORY");
  const issueToken = required("GITHUB_TOKEN");
  const runUrl = required("AGENT_RUN_URL");
  const resumeKind = required("RESUME_KIND");
  const updates = { "Agent run": runUrl };
  if (resumeKind === "small") Object.assign(updates, { Track: "Small", Status: "Human review" });
  else if (resumeKind === "exec_plan") {
    Object.assign(updates, { Track: "Exec Plan", Status: "Human review" });
  } else if (resumeKind === "needs_input") Object.assign(updates, { Status: "Needs input" });
  else throw new Error(`unsupported resume kind: ${resumeKind}`);

  try {
    await syncProjectItem({
      token: required("AGENT_PROJECT_TOKEN"),
      owner: required("AGENT_PROJECT_OWNER"),
      number: positiveInteger(required("AGENT_PROJECT_NUMBER"), "AGENT_PROJECT_NUMBER"),
      contentId: required("CONTENT_ID"),
      updates,
    });
  } catch (error) {
    await bestEffortDiagnostic({
      issueToken,
      repository,
      issueNumber,
      stage: "retrying the published triage",
      error,
      runUrl,
    });
    throw error;
  }
}

async function bestEffortDiagnostic({ issueToken, repository, issueNumber, stage, error, runUrl }) {
  try {
    await upsertManagedComment({
      token: issueToken,
      repository,
      issueNumber,
      markerPrefix: DIAGNOSTIC_MARKER,
      body: renderDiagnostic({ issueNumber, stage, message: messageOf(error), runUrl }),
    });
  } catch (commentError) {
    process.stderr.write(`Could not publish diagnostic: ${messageOf(commentError)}\n`);
  }
}

function required(name) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function positiveInteger(value, name) {
  const number = Number(value);
  if (!Number.isSafeInteger(number) || number <= 0) {
    throw new Error(`${name} must be a positive integer`);
  }
  return number;
}

function writeOutput(name, value) {
  appendFileSync(required("GITHUB_OUTPUT"), `${name}=${value}\n`, "utf8");
}

function messageOf(error) {
  return error instanceof Error ? error.message : String(error);
}
