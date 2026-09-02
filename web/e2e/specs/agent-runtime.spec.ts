import type { components } from "../../src/lib/http-commons/schema.d.ts";
import { createHash } from "node:crypto";
import { expect, test as base } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import {
  AGENT_COMMITTED_RECEIPT,
  AGENT_CONFIRMATION_RESPONSE,
  AGENT_FIXTURE_MODEL,
  AGENT_OCR_RESPONSE,
  AGENT_PLAIN_RESPONSE,
  AGENT_PROVIDER_PRIVATE_MARKER,
  AGENT_REJECTED_RECEIPT,
  AGENT_REJECTION_RESPONSE,
  agentAPIResponse,
  agentModelMetrics,
  agentScenarioPrompt,
  agentStreamFacts,
  agentStreamOutput,
  albumAssets,
  e2eServerLogsSince,
  prepareAgentAlbumFixture,
  prepareAgentOCRFixture,
} from "../support/agentRuntime";
import { api } from "../support/api";
import { t } from "../support/i18n";
import { provisionWorkspace, type Workspace } from "../support/workspace";

type Settings = components["schemas"]["dto.SystemSettingsDTO"];
type Capabilities = components["schemas"]["dto.CapabilitiesResponseDTO"];
type EffectStatus = components["schemas"]["handler.AgentEffectStatusResponse"];

type RuntimeWorkspace = Workspace & { runtimeIndex: number };

const test = base.extend<{ runtimeWorkspace: RuntimeWorkspace }>({
  runtimeWorkspace: async ({ browserName }, use, testInfo) => {
    // The shared E2E fixture is worker-scoped, but this slice deliberately owns
    // one user and repository per test. Include repeat/retry identity so the
    // three-pass stability run cannot inherit a prior scenario's server state.
    const identity = [browserName, testInfo.testId, testInfo.repeatEachIndex, testInfo.retry].join(
      ":",
    );
    const slot = Number.parseInt(
      createHash("sha256").update(identity).digest("hex").slice(0, 10),
      16,
    );
    const runtimeIndex = slot * 10;
    await use({ ...(await provisionWorkspace(runtimeIndex)), runtimeIndex });
  },
});

// `runtimeWorkspace` is test-scoped, so it is created before a test body can
// call `test.setTimeout()`. Keep the complete setup and assertion budget at
// this file scope; the Radxa qualification host can legitimately spend more
// than the default thirty seconds creating an isolated user and repository.
test.describe.configure({ timeout: 120_000 });

test.beforeEach(async ({ runtimeWorkspace: workspace }) => {
  const settings = await api<Settings>("/api/v1/settings/system", { token: workspace.token });
  expect(settings.llm).toMatchObject({
    agent_enabled: true,
    provider: "ollama",
    model_name: AGENT_FIXTURE_MODEL,
    api_key_configured: false,
  });
  const capabilities = await api<Capabilities>("/api/v1/capabilities", {
    token: workspace.token,
  });
  expect(capabilities.llm).toMatchObject({
    availability: "ready",
    agent_enabled: true,
    configured: true,
    provider: "ollama",
    model_name: AGENT_FIXTURE_MODEL,
  });
});

test("@agent-runtime context-free chat crosses the real keyless SSE runtime", async ({
  page,
  runtimeWorkspace: workspace,
}) => {
  test.setTimeout(90_000);
  const before = await agentModelMetrics();

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");
  await expect(page.getByText(t("lumilio.dock.ready"), { exact: true })).toBeVisible();

  const chatResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "plain" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();

  const response = await chatResponse;
  expect(response.status()).toBe(200);
  expect(response.headers()["content-type"]).toContain("text/event-stream");
  await expect(page.getByText(AGENT_PLAIN_RESPONSE, { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: t("lumilio.input.send"), exact: true }),
  ).toBeVisible();

  const after = await agentModelMetrics();
  expect(after.plain_completed).toBe(before.plain_completed + 1);
  expect(after.auth_rejections).toBe(before.auth_rejections);
  expect(after.protocol_errors).toBe(before.protocol_errors);
});

test("@agent-runtime viewer context reads stored OCR without crossing owners", async ({
  page,
  runtimeWorkspace: workspace,
}, testInfo) => {
  test.setTimeout(120_000);
  const fixture = await prepareAgentOCRFixture(
    workspace,
    `${Date.now()}-${testInfo.repeatEachIndex}-${testInfo.retry}`,
  );
  const before = await agentModelMetrics();

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto(`/assets/${fixture.assetId}`);
  const askFromViewer = page.getByRole("button", {
    name: t("lumilio.viewer.ask"),
    exact: true,
  });
  await expect(askFromViewer).toBeVisible({ timeout: 30_000 });
  await askFromViewer.click();
  await expect(page.getByText(t("lumilio.context.viewing"), { exact: true })).toBeVisible();

  const chatResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "read-ocr" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();

  const response = await chatResponse;
  expect(response.status()).toBe(200);
  await expect(page.getByText(AGENT_OCR_RESPONSE, { exact: true })).toBeVisible();
  const afterRead = await agentModelMetrics();
  expect(afterRead.read_ocr_call).toBe(before.read_ocr_call + 1);
  expect(afterRead.read_ocr_final).toBe(before.read_ocr_final + 1);
  expect(afterRead.protocol_errors).toBe(before.protocol_errors);

  const other = await provisionWorkspace(workspace.runtimeIndex + 1);
  const crossOwner = await agentAPIResponse(other.token, "/api/v1/agent/chat", {
    method: "POST",
    body: {
      query: agentScenarioPrompt({ name: "read-ocr" }),
      mode: "free",
      context: [
        {
          type: "viewing",
          label: t("lumilio.context.viewing"),
          asset_ids: [fixture.assetId],
        },
      ],
    },
  });
  expect(crossOwner.status).not.toBe(200);
  expect(await crossOwner.text()).not.toContain(fixture.lines.join(" "));
  const afterIsolationAttempt = await agentModelMetrics();
  expect(afterIsolationAttempt.read_ocr_call).toBe(afterRead.read_ocr_call);
  expect(afterIsolationAttempt.read_ocr_final).toBe(afterRead.read_ocr_final);
});

test("@agent-runtime approved album mutation commits exactly once through resume", async ({
  page,
  runtimeWorkspace: workspace,
}, testInfo) => {
  test.setTimeout(120_000);
  const fixture = await prepareAgentAlbumFixture(
    workspace,
    `${Date.now()}-${testInfo.repeatEachIndex}-${testInfo.retry}`,
  );
  const before = await agentModelMetrics();

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");
  const chatResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(
    agentScenarioPrompt({
      name: "confirm-add-to-album",
      filename: fixture.filename,
      album_title: fixture.albumTitle,
    }),
  );
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  const initial = await chatResponse;
  expect(initial.status()).toBe(200);
  expect(initial.headers()["content-type"]).toContain("text/event-stream");
  await expect(page.getByText(t("lumilio.chat.confirmation.title"), { exact: true })).toBeVisible();

  const resumeResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat/resume",
  );
  await page
    .getByRole("button", { name: t("lumilio.chat.confirmation.confirm"), exact: true })
    .click();
  const resumed = await resumeResponse;
  expect(resumed.status()).toBe(200);
  expect(resumed.headers()["content-type"]).toContain("text/event-stream");
  await expect(
    page.getByRole("status").getByText(AGENT_COMMITTED_RECEIPT, { exact: true }),
  ).toBeVisible();
  await expect(page.getByText(AGENT_CONFIRMATION_RESPONSE, { exact: true })).toBeVisible();

  await expect
    .poll(async () => {
      const result = await albumAssets(workspace.token, fixture.albumId);
      return result.assets?.map((asset) => asset.asset_id) ?? [];
    })
    .toEqual([fixture.assetId]);

  const after = await agentModelMetrics();
  expect(after.confirmation_lookup).toBe(before.confirmation_lookup + 1);
  expect(after.confirmation_filter).toBe(before.confirmation_filter + 1);
  expect(after.confirmation_add).toBe(before.confirmation_add + 1);
  expect(after.confirmation_final).toBe(before.confirmation_final + 1);
  expect(after.auth_rejections).toBe(before.auth_rejections);
  expect(after.protocol_errors).toBe(before.protocol_errors);
});

test("@agent-runtime rejected album mutation records one receipt and changes nothing", async ({
  page,
  runtimeWorkspace: workspace,
}, testInfo) => {
  test.setTimeout(120_000);
  const fixture = await prepareAgentAlbumFixture(
    workspace,
    `reject-${Date.now()}-${testInfo.repeatEachIndex}-${testInfo.retry}`,
  );
  const before = await agentModelMetrics();

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");
  const chatResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(
    agentScenarioPrompt({
      name: "confirm-add-to-album",
      filename: fixture.filename,
      album_title: fixture.albumTitle,
    }),
  );
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  const initial = await chatResponse;
  expect(initial.status()).toBe(200);
  await expect(page.getByText(t("lumilio.chat.confirmation.title"), { exact: true })).toBeVisible();
  const facts = agentStreamFacts(await initial.text());
  expect(facts.interruptId).toBeTruthy();
  expect(facts.effectId).toBeTruthy();

  const resumeResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat/resume",
  );
  await page.getByRole("button", { name: t("common.cancel"), exact: true }).click();
  const resumed = await resumeResponse;
  expect(resumed.status()).toBe(200);
  await expect(
    page.getByRole("status").getByText(AGENT_REJECTED_RECEIPT, { exact: true }),
  ).toBeVisible();
  await expect(page.getByText(AGENT_REJECTION_RESPONSE, { exact: true })).toBeVisible();

  await expect
    .poll(async () => (await albumAssets(workspace.token, fixture.albumId)).count)
    .toBe(0);
  const effect = await api<EffectStatus>(
    `/api/v1/agent/effects/${facts.effectId}?thread_id=${encodeURIComponent(facts.threadId)}`,
    { token: workspace.token },
  );
  expect(effect).toMatchObject({ status: "rejected", receipt: { status: "rejected" } });

  const after = await agentModelMetrics();
  expect(after.confirmation_rejected).toBe(before.confirmation_rejected + 1);
  expect(after.confirmation_final).toBe(before.confirmation_final + 1);
  expect(after.auth_rejections).toBe(before.auth_rejections);
  expect(after.protocol_errors).toBe(before.protocol_errors);
});

test("@agent-runtime a stale duplicate confirmation cannot repeat the mutation", async ({
  page,
  runtimeWorkspace: workspace,
}, testInfo) => {
  test.setTimeout(120_000);
  const fixture = await prepareAgentAlbumFixture(
    workspace,
    `stale-${Date.now()}-${testInfo.repeatEachIndex}-${testInfo.retry}`,
  );
  const before = await agentModelMetrics();

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");
  const chatResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(
    agentScenarioPrompt({
      name: "confirm-add-to-album",
      filename: fixture.filename,
      album_title: fixture.albumTitle,
    }),
  );
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  const initial = await chatResponse;
  await expect(page.getByText(t("lumilio.chat.confirmation.title"), { exact: true })).toBeVisible();
  const facts = agentStreamFacts(await initial.text());
  expect(facts.interruptId).toBeTruthy();
  expect(facts.effectId).toBeTruthy();

  const resumeResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat/resume",
  );
  await page
    .getByRole("button", { name: t("lumilio.chat.confirmation.confirm"), exact: true })
    .click();
  expect((await resumeResponse).status()).toBe(200);
  await expect(page.getByText(AGENT_CONFIRMATION_RESPONSE, { exact: true })).toBeVisible();

  const stale = await agentAPIResponse(workspace.token, "/api/v1/agent/chat/resume", {
    method: "POST",
    body: {
      thread_id: facts.threadId,
      targets: { [facts.interruptId!]: { approved: true } },
    },
  });
  expect(stale.status).toBe(404);
  const effect = await api<EffectStatus>(
    `/api/v1/agent/effects/${facts.effectId}?thread_id=${encodeURIComponent(facts.threadId)}`,
    { token: workspace.token },
  );
  expect(effect).toMatchObject({ status: "committed", receipt: { count: 1, status: "committed" } });
  await expect
    .poll(async () => {
      const result = await albumAssets(workspace.token, fixture.albumId);
      return result.assets?.map((asset) => asset.asset_id) ?? [];
    })
    .toEqual([fixture.assetId]);

  const after = await agentModelMetrics();
  expect(after.confirmation_final).toBe(before.confirmation_final + 1);
  expect(after.protocol_errors).toBe(before.protocol_errors);
});

test("@agent-runtime stop cancels the exact slow run and permits a later message", async ({
  page,
  runtimeWorkspace: workspace,
}) => {
  test.setTimeout(120_000);
  const before = await agentModelMetrics();
  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");

  const chatResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "slow-stream" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  const slow = await chatResponse;
  expect(slow.status()).toBe(200);
  await expect
    .poll(async () => (await agentModelMetrics()).slow_started)
    .toBe(before.slow_started + 1);

  const cancelResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat/cancel",
  );
  await page.getByRole("button", { name: t("lumilio.input.stop"), exact: true }).click();
  const cancelled = await cancelResponse;
  expect(cancelled.status()).toBe(200);
  // The handler returns the exact tuple it parsed only after the scoped
  // user/thread/run update succeeds. Playwright does not retain postData for
  // every native Fetch request, so this response is the authoritative public
  // proof that Stop targeted a concrete active run rather than a thread-wide
  // cancellation fallback.
  const cancelResult = (await cancelled.json()) as {
    thread_id: string;
    run_id: string;
    status: string;
  };
  expect(cancelResult.thread_id).not.toBe("");
  expect(cancelResult.run_id).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(cancelResult.status).toMatch(/cancel_requested|cancelled/);
  await expect(page.getByText(t("lumilio.messages.stopped"), { exact: true })).toBeVisible();
  await expect
    .poll(async () => (await agentModelMetrics()).slow_cancelled)
    .toBe(before.slow_cancelled + 1);

  const recoveryResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "plain" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  expect((await recoveryResponse).status()).toBe(200);
  await expect(page.getByText(AGENT_PLAIN_RESPONSE, { exact: true })).toBeVisible();

  const after = await agentModelMetrics();
  expect(after.plain_completed).toBe(before.plain_completed + 1);
  expect(after.protocol_errors).toBe(before.protocol_errors);
});

test("@agent-runtime provider failure is sanitized and releases the thread for recovery", async ({
  page,
  runtimeWorkspace: workspace,
}) => {
  test.setTimeout(120_000);
  const before = await agentModelMetrics();
  const logStart = new Date(Date.now() - 1_000);
  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");

  const failureResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "provider-error" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  const failed = await failureResponse;
  expect(failed.status()).toBe(200);
  await expect(page.getByText(t("apiErrors.agent.operationFailed"), { exact: true })).toBeVisible();
  expect(await failed.text()).not.toContain(AGENT_PROVIDER_PRIVATE_MARKER);
  expect(await page.getByText(AGENT_PROVIDER_PRIVATE_MARKER, { exact: true }).count()).toBe(0);
  expect(e2eServerLogsSince(logStart)).not.toContain(AGENT_PROVIDER_PRIVATE_MARKER);

  const recoveryResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/agent/chat",
  );
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "plain" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  expect((await recoveryResponse).status()).toBe(200);
  await expect(page.getByText(AGENT_PLAIN_RESPONSE, { exact: true })).toBeVisible();

  const after = await agentModelMetrics();
  expect(after.provider_errors).toBe(before.provider_errors + 1);
  expect(after.plain_completed).toBe(before.plain_completed + 1);
  expect(after.protocol_errors).toBe(before.protocol_errors);
});

test("@agent-runtime confirmation identities are isolated between authenticated users", async ({
  runtimeWorkspace: workspace,
}, testInfo) => {
  test.setTimeout(120_000);
  const fixture = await prepareAgentAlbumFixture(
    workspace,
    `isolation-${Date.now()}-${testInfo.repeatEachIndex}-${testInfo.retry}`,
  );
  const other = await provisionWorkspace(workspace.runtimeIndex + 1);
  const initial = await agentAPIResponse(workspace.token, "/api/v1/agent/chat", {
    method: "POST",
    body: {
      query: agentScenarioPrompt({
        name: "confirm-add-to-album",
        filename: fixture.filename,
        album_title: fixture.albumTitle,
      }),
      mode: "free",
    },
  });
  expect(initial.status).toBe(200);
  const facts = agentStreamFacts(await initial.text());
  expect(facts.interruptId).toBeTruthy();
  expect(facts.effectId).toBeTruthy();

  const crossUserCancel = await agentAPIResponse(other.token, "/api/v1/agent/chat/cancel", {
    method: "POST",
    body: { thread_id: facts.threadId, run_id: facts.runId },
  });
  expect(crossUserCancel.status).toBe(404);
  const crossUserResume = await agentAPIResponse(other.token, "/api/v1/agent/chat/resume", {
    method: "POST",
    body: {
      thread_id: facts.threadId,
      targets: { [facts.interruptId!]: { approved: true } },
    },
  });
  expect(crossUserResume.status).toBe(404);
  const crossUserEffect = await agentAPIResponse(
    other.token,
    `/api/v1/agent/effects/${facts.effectId}?thread_id=${encodeURIComponent(facts.threadId)}`,
  );
  expect(crossUserEffect.status).toBe(404);

  const cleanup = await agentAPIResponse(workspace.token, "/api/v1/agent/chat/resume", {
    method: "POST",
    body: {
      thread_id: facts.threadId,
      targets: { [facts.interruptId!]: { approved: false } },
    },
  });
  expect(cleanup.status).toBe(200);
  expect(agentStreamOutput(await cleanup.text())).toBe(AGENT_REJECTION_RESPONSE);
  await expect
    .poll(async () => (await albumAssets(workspace.token, fixture.albumId)).count)
    .toBe(0);
});
