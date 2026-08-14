import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../../../", import.meta.url);

test("quick capture is privacy-first and does not self-enroll", async () => {
  const form = await read(".github/ISSUE_TEMPLATE/quick-task.yml");
  assert.match(form, /quick-capture/);
  assert.doesNotMatch(form, /^\s*- agent-task\s*$/m);
  assert.match(form, /original media/);
  assert.match(form, /database files/);
  assert.match(form, /unsanitized logs/);
});

test("triage prompt makes repository instructions authoritative and writes no code", async () => {
  const prompt = await read(".github/codex/prompts/triage.md");
  assert.match(prompt, /read-only preflight/);
  assert.match(prompt, /Repository instructions outrank/);
  assert.match(prompt, /Do not implement, edit files, create commits/);
  assert.match(prompt, /exec-plans\/active\//);
});

test("intake prepares a subscription-backed Cloud handoff without API billing", async () => {
  const intake = await read(".github/workflows/agent-intake.yml");
  const command = await read(".github/workflows/agent-command.yml");
  const triage = await read(".github/codex/scripts/triage.mjs");
  assert.match(intake, /agent-loop\.mjs prepare-cloud-handoff/);
  assert.match(intake, /workflow_dispatch:/);
  assert.match(intake, /ref: \$\{\{ vars\.AGENT_BASE_BRANCH \|\| 'dev' \}\}/);
  assert.match(command, /agent-loop\.mjs publish-comment-triage/);
  assert.match(command, /ref: \$\{\{ vars\.AGENT_BASE_BRANCH \|\| 'dev' \}\}/);
  assert.match(triage, /https:\/\/chatgpt\.com\/codex/);
  assert.match(triage, /did not call a model/);
  assert.match(triage, /\/agent-submit/);
  assert.doesNotMatch(`${intake}\n${command}`, /OPENAI_API_KEY|openai-api-key|codex-action/);
});

test("CI owns a path-filtered local policy gate", async () => {
  const taskfile = await read("taskfile.yml");
  const workflow = await read(".github/workflows/ci.yml");
  assert.match(taskfile, /^  agent-loop:test:/m);
  assert.match(workflow, /^                      agent_loop:/m);
  assert.match(workflow, /^    agent-loop:/m);
  assert.match(workflow, /task agent-loop:test/);
  assert.match(workflow, /needs\.agent-loop\.result/);
});

async function read(path) {
  return readFile(new URL(path, root), "utf8");
}
