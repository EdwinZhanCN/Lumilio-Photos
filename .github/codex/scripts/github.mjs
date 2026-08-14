import {
  commandDecision,
  isTrustedActor,
  parseAgentCommand,
  sanitizePromptText,
} from "./policy.mjs";
import {
  CONTRACT_MARKER,
  NEEDS_INPUT_MARKER,
  PLAN_REQUEST_MARKER,
  TRIAGE_MARKERS,
} from "./triage.mjs";

export function parseRepository(value) {
  if (typeof value !== "string" || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(value)) {
    throw new Error("GITHUB_REPOSITORY must be owner/name");
  }
  const [owner, name] = value.split("/");
  return { owner, name, fullName: value };
}

export async function authorizeIntake({
  eventName,
  event,
  repository,
  actorLogin,
  manualIssueNumber,
  token,
  fetchImpl = fetch,
}) {
  const repo = parseRepository(repository);
  const issueNumber =
    eventName === "workflow_dispatch"
      ? Number(manualIssueNumber)
      : Number(event?.issue?.number);
  if (!Number.isSafeInteger(issueNumber) || issueNumber <= 0) {
    throw new Error("a positive Issue number is required");
  }

  const issue =
    event?.issue?.number === issueNumber
      ? event.issue
      : await githubRest({ token, repository, path: `/issues/${issueNumber}`, fetchImpl });
  if (issue.pull_request) throw new Error("the intake target must be an Issue");

  const actor =
    eventName === "workflow_dispatch"
      ? actorLogin
      : event?.sender?.login;
  const actorType = eventName === "workflow_dispatch" ? event?.sender?.type ?? "User" : event?.sender?.type;
  if (!actor) throw new Error("the triggering actor is missing");
  if (actorType === "Bot") {
    return {
      authorized: false,
      reason: "untrusted-actor",
      issueNumber,
      contentId: issue.node_id,
      actor,
      issue,
    };
  }

  const permission = await repositoryPermission({
    token,
    repository,
    actor,
    fetchImpl,
  });
  const association =
    actor === repo.owner
      ? "OWNER"
      : actor === issue.user?.login && issue.author_association
        ? issue.author_association
        : permission === "write" || permission === "maintain" || permission === "admin"
          ? "COLLABORATOR"
          : "NONE";
  const trustedActor = isTrustedActor({ permission, association, actorType });

  const issueLabels = (issue.labels ?? []).map((label) =>
    typeof label === "string" ? label : label.name,
  );
  const action = eventName === "workflow_dispatch" ? "dispatch" : event.action;
  const label = event?.label?.name;

  let reason = "not-enrolled";
  let authorized = false;
  if (!trustedActor) {
    reason = "untrusted-actor";
  } else if (eventName === "workflow_dispatch") {
    authorized = true;
    reason = "manual-recovery";
  } else if (
    eventName === "issues" &&
    (action === "opened" || action === "reopened") &&
    issueLabels.includes("quick-capture")
  ) {
    authorized = true;
    reason = "trusted-quick-capture";
  } else if (eventName === "issues" && action === "labeled" && label === "agent-task") {
    authorized = true;
    reason = "trusted-enrollment";
  }

  return {
    authorized,
    reason,
    issueNumber,
    contentId: issue.node_id,
    actor,
    issue,
  };
}

export async function authorizeCommand({
  eventName,
  event,
  repository,
  actorLogin,
  manualIssueNumber,
  manualCommand,
  token,
  fetchImpl = fetch,
}) {
  const repo = parseRepository(repository);
  const body =
    eventName === "workflow_dispatch"
      ? `/agent-${manualCommand}`
      : event?.comment?.body;
  const parsed = parseAgentCommand(body);
  if (!parsed) return { authorized: false, reason: "not-a-command" };
  if (parsed.command === "run") {
    return { authorized: false, reason: "implementation-disabled" };
  }

  const issueNumber =
    eventName === "workflow_dispatch"
      ? Number(manualIssueNumber)
      : Number(event?.issue?.number);
  if (!Number.isSafeInteger(issueNumber) || issueNumber <= 0) {
    throw new Error("a positive Issue number is required");
  }
  const issue =
    event?.issue?.number === issueNumber
      ? event.issue
      : await githubRest({ token, repository, path: `/issues/${issueNumber}`, fetchImpl });
  if (issue.pull_request) {
    return { authorized: false, reason: "pull-request-commands-not-enabled", issueNumber };
  }

  const actor = eventName === "workflow_dispatch" ? actorLogin : event?.sender?.login;
  const actorType = eventName === "workflow_dispatch" ? event?.sender?.type ?? "User" : event?.sender?.type;
  if (!actor) throw new Error("the triggering actor is missing");
  if (actorType === "Bot") {
    return { authorized: false, reason: "untrusted-actor", issueNumber };
  }

  const permission = await repositoryPermission({
    token,
    repository,
    actor,
    fetchImpl,
  });
  const association =
    actor === repo.owner
      ? "OWNER"
      : permission === "write" || permission === "maintain" || permission === "admin"
        ? "COLLABORATOR"
        : "NONE";
  const trustedActor = isTrustedActor({ permission, association, actorType });
  const issueLabels = (issue.labels ?? []).map((label) =>
    typeof label === "string" ? label : label.name,
  );
  const decision = commandDecision({
    body,
    trustedActor,
    actorType,
    subjectKind: "issue",
    issueLabels,
  });
  if (decision.prepareHandoff && decision.command === "submit" && decision.feedback === "") {
    return { authorized: false, reason: "missing-triage-json", issueNumber };
  }
  let resume = "";
  if (decision.prepareHandoff && decision.command === "retry") {
    const comments = await listIssueComments({ token, repository, issueNumber, fetchImpl });
    const managed = comments.filter(
      (comment) =>
        comment.user?.login === "github-actions[bot]" &&
        TRIAGE_MARKERS.some((markerPrefix) => comment.body?.startsWith(markerPrefix)),
    );
    if (managed.length > 1) {
      return { authorized: false, reason: "multiple-managed-contracts", issueNumber };
    }
    if (managed[0]?.body?.startsWith(CONTRACT_MARKER)) resume = "small";
    else if (managed[0]?.body?.startsWith(PLAN_REQUEST_MARKER)) resume = "exec_plan";
    else if (managed[0]?.body?.startsWith(NEEDS_INPUT_MARKER)) resume = "needs_input";
  }
  return {
    authorized: decision.prepareHandoff,
    reason: decision.reason,
    issueNumber,
    contentId: issue.node_id,
    mode: decision.command ?? parsed.command,
    commentId: event?.comment?.id ?? null,
    resume,
  };
}

export async function ensureIssueLabel({ token, repository, issueNumber, label, fetchImpl = fetch }) {
  return githubRest({
    token,
    repository,
    path: `/issues/${issueNumber}/labels`,
    method: "POST",
    body: { labels: [label] },
    fetchImpl,
  });
}

export async function listIssueComments({ token, repository, issueNumber, fetchImpl = fetch }) {
  const comments = [];
  for (let page = 1; page <= 20; page += 1) {
    const batch = await githubRest({
      token,
      repository,
      path: `/issues/${issueNumber}/comments?per_page=100&page=${page}`,
      fetchImpl,
    });
    comments.push(...batch);
    if (batch.length < 100) return comments;
  }
  throw new Error("managed Issue has more than 2000 comments; refusing an incomplete scan");
}

export async function upsertManagedComment({
  token,
  repository,
  issueNumber,
  markerPrefix,
  body,
  fetchImpl = fetch,
}) {
  const comments = await listIssueComments({ token, repository, issueNumber, fetchImpl });
  const managed = comments.filter(
    (comment) =>
      comment.user?.login === "github-actions[bot]" &&
      typeof comment.body === "string" &&
      comment.body.startsWith(markerPrefix),
  );
  if (managed.length > 1) {
    throw new Error(`multiple managed comments found for ${markerPrefix}`);
  }

  if (managed.length === 1) {
    return githubRest({
      token,
      repository,
      path: `/issues/comments/${managed[0].id}`,
      method: "PATCH",
      body: { body },
      fetchImpl,
    });
  }
  return githubRest({
    token,
    repository,
    path: `/issues/${issueNumber}/comments`,
    method: "POST",
    body: { body },
    fetchImpl,
  });
}

export async function deleteIssueComment({ token, repository, commentId, fetchImpl = fetch }) {
  return githubRest({
    token,
    repository,
    path: `/issues/comments/${commentId}`,
    method: "DELETE",
    fetchImpl,
  });
}

export async function listOpenIssueIndex({ token, repository, excludeIssue, fetchImpl = fetch }) {
  const issues = await githubRest({
    token,
    repository,
    path: "/issues?state=open&sort=updated&direction=desc&per_page=100",
    fetchImpl,
  });
  return issues
    .filter((issue) => !issue.pull_request && issue.number !== excludeIssue)
    .slice(0, 50)
    .map((issue) => ({
      number: issue.number,
      title: sanitizePromptText(issue.title, 300),
      labels: (issue.labels ?? []).map((label) => sanitizePromptText(label.name ?? label, 80)),
    }));
}

async function repositoryPermission({ token, repository, actor, fetchImpl }) {
  try {
    const payload = await githubRest({
      token,
      repository,
      path: `/collaborators/${encodeURIComponent(actor)}/permission`,
      fetchImpl,
    });
    return payload.permission;
  } catch (error) {
    // Public reporters may not be collaborators. Treat absence as no authority
    // instead of turning an ordinary public Issue into a failed workflow run.
    if (error?.status === 404) return "none";
    throw error;
  }
}

export async function githubRest({
  token,
  repository,
  path,
  method = "GET",
  body,
  fetchImpl = fetch,
}) {
  if (!token) throw new Error("a GitHub token is required");
  parseRepository(repository);
  const response = await fetchImpl(`https://api.github.com/repos/${repository}${path}`, {
    method,
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      "User-Agent": "lumilio-agent-loop",
      "X-GitHub-Api-Version": "2022-11-28",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  const text = await response.text();
  const payload = text === "" ? null : JSON.parse(text);
  if (!response.ok) {
    const message = payload?.message ?? response.statusText;
    const error = new Error(
      `GitHub REST ${method} ${path} failed (${response.status}): ${message}`,
    );
    error.status = response.status;
    throw error;
  }
  return payload;
}
