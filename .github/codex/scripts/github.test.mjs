import assert from "node:assert/strict";
import test from "node:test";

import { authorizeCommand, authorizeIntake, parseRepository } from "./github.mjs";

function response(payload, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "error",
    async text() {
      return JSON.stringify(payload);
    },
  };
}

test("parses only a bounded owner/name repository", () => {
  assert.deepEqual(parseRepository("owner/repo"), {
    owner: "owner",
    name: "repo",
    fullName: "owner/repo",
  });
  assert.throws(() => parseRepository("owner/repo/extra"));
  assert.throws(() => parseRepository("https://github.com/owner/repo"));
});

test("authorizes a trusted quick capture", async () => {
  const event = {
    action: "opened",
    sender: { login: "owner", type: "User" },
    issue: {
      number: 42,
      node_id: "issue-node",
      user: { login: "owner" },
      author_association: "OWNER",
      labels: [{ name: "quick-capture" }],
    },
  };
  const result = await authorizeIntake({
    eventName: "issues",
    event,
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl: async () => response({ permission: "admin" }),
  });
  assert.equal(result.authorized, true);
  assert.equal(result.reason, "trusted-quick-capture");
});

test("a bot label event and an untrusted public Issue cannot authorize model spend", async () => {
  const baseEvent = {
    action: "labeled",
    label: { name: "agent-task" },
    sender: { login: "github-actions[bot]", type: "Bot" },
    issue: {
      number: 42,
      node_id: "issue-node",
      user: { login: "reporter" },
      author_association: "NONE",
      labels: [{ name: "agent-task" }],
    },
  };
  let botFetches = 0;
  const bot = await authorizeIntake({
    eventName: "issues",
    event: baseEvent,
    repository: "owner/repo",
    actorLogin: "github-actions[bot]",
    token: "token",
    fetchImpl: async () => {
      botFetches += 1;
      return response({ permission: "admin" });
    },
  });
  assert.equal(bot.authorized, false);
  assert.equal(botFetches, 0);

  const publicIssue = await authorizeIntake({
    eventName: "issues",
    event: {
      ...baseEvent,
      action: "opened",
      sender: { login: "reporter", type: "User" },
      issue: { ...baseEvent.issue, labels: [{ name: "quick-capture" }] },
    },
    repository: "owner/repo",
    actorLogin: "reporter",
    token: "token",
    fetchImpl: async () => response({ permission: "read" }),
  });
  assert.equal(publicIssue.authorized, false);
  assert.equal(publicIssue.reason, "untrusted-actor");

  const nonCollaborator = await authorizeIntake({
    eventName: "issues",
    event: {
      ...baseEvent,
      action: "opened",
      sender: { login: "outside-reporter", type: "User" },
      issue: { ...baseEvent.issue, labels: [{ name: "quick-capture" }] },
    },
    repository: "owner/repo",
    actorLogin: "outside-reporter",
    token: "token",
    fetchImpl: async () => response({ message: "Not Found" }, 404),
  });
  assert.equal(nonCollaborator.authorized, false);
  assert.equal(nonCollaborator.reason, "untrusted-actor");
});

test("authorizes only trusted revise/retry commands on managed Issues", async () => {
  const event = {
    sender: { login: "owner", type: "User" },
    comment: { id: 123, body: "/agent-revise\nKeep validation observable." },
    issue: {
      number: 42,
      node_id: "issue-node",
      labels: [{ name: "agent-task" }],
    },
  };
  const result = await authorizeCommand({
    eventName: "issue_comment",
    event,
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl: async () => response({ permission: "admin" }),
  });
  assert.equal(result.authorized, true);
  assert.equal(result.mode, "revise");
  assert.equal(result.commentId, 123);

  const retry = await authorizeCommand({
    eventName: "workflow_dispatch",
    event: { sender: { login: "owner", type: "User" } },
    repository: "owner/repo",
    actorLogin: "owner",
    manualIssueNumber: "42",
    manualCommand: "retry",
    token: "token",
    fetchImpl: async (url) => {
      if (url.includes("/issues/42/comments")) return response([]);
      if (url.includes("/issues/42")) {
        return response({ number: 42, node_id: "issue-node", labels: [{ name: "agent-task" }] });
      }
      return response({ permission: "admin" });
    },
  });
  assert.equal(retry.authorized, true);
  assert.equal(retry.mode, "retry");
  assert.equal(retry.resume, "");

  const resume = await authorizeCommand({
    eventName: "issue_comment",
    event: { ...event, comment: { id: 456, body: "/agent-retry" } },
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl: async (url) =>
      url.includes("/comments")
        ? response([
            {
              user: { login: "github-actions[bot]" },
              body: "<!-- lumilio-agent-contract:v1 issue=42 revision=3 -->\ncontract",
            },
          ])
        : response({ permission: "admin" }),
  });
  assert.equal(resume.authorized, true);
  assert.equal(resume.resume, "small");
});

test("authorizes a trusted returned Cloud preflight only when JSON follows the command", async () => {
  const base = {
    sender: { login: "owner", type: "User" },
    issue: { number: 42, node_id: "issue-node", labels: [{ name: "agent-task" }] },
  };
  const accepted = await authorizeCommand({
    eventName: "issue_comment",
    event: {
      ...base,
      comment: { id: 789, body: '/agent-submit\n{"track":"small"}' },
    },
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl: async () => response({ permission: "admin" }),
  });
  assert.equal(accepted.authorized, true);
  assert.equal(accepted.mode, "submit");
  assert.equal(accepted.commentId, 789);

  const missing = await authorizeCommand({
    eventName: "issue_comment",
    event: { ...base, comment: { id: 790, body: "/agent-submit" } },
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl: async () => response({ permission: "admin" }),
  });
  assert.equal(missing.authorized, false);
  assert.equal(missing.reason, "missing-triage-json");
});

test("does not spend on run, lookalike, bot, unmanaged, or pull-request commands", async () => {
  const base = {
    sender: { login: "owner", type: "User" },
    comment: { body: "/agent-run" },
    issue: { number: 42, node_id: "issue-node", labels: [{ name: "agent-task" }] },
  };
  let fetches = 0;
  const fetchImpl = async () => {
    fetches += 1;
    return response({ permission: "admin" });
  };
  const disabled = await authorizeCommand({
    eventName: "issue_comment",
    event: base,
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl,
  });
  assert.equal(disabled.reason, "implementation-disabled");
  assert.equal(fetches, 0);

  const lookalike = await authorizeCommand({
    eventName: "issue_comment",
    event: { ...base, comment: { body: "/agent-run now" } },
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl,
  });
  assert.equal(lookalike.reason, "not-a-command");
  assert.equal(fetches, 0);

  const pullRequest = await authorizeCommand({
    eventName: "issue_comment",
    event: { ...base, comment: { body: "/agent-retry" }, issue: { ...base.issue, pull_request: {} } },
    repository: "owner/repo",
    actorLogin: "owner",
    token: "token",
    fetchImpl,
  });
  assert.equal(pullRequest.reason, "pull-request-commands-not-enabled");
  assert.equal(fetches, 0);
});
