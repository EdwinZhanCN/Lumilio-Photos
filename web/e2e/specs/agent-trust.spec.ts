import type { Route } from "playwright/test";
import { test, expect } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { t } from "../support/i18n";

const json = (route: Route, body: unknown) =>
  route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });

const sse = (...events: { event: string; data: unknown }[]) =>
  `${events.map(({ event, data }) => `event: ${event}\ndata: ${JSON.stringify(data)}\n`).join("\n")}\n`;

test("@agent-trust Lumilio Agent exposes scope and waits for an effect receipt", async ({
  page,
  workspace,
}) => {
  test.setTimeout(60_000);

  await page.route("**/api/v1/capabilities", (route) =>
    json(route, {
      ml: {
        discovered_node_count: 0,
        active_node_count: 0,
        tasks: {
          semantic_image_embed: { enabled: false, available: false },
          semantic_text_embed: { enabled: false, available: false },
          bioclip_classify: { enabled: false, available: false },
          ocr: { enabled: false, available: false },
          face_recognition: { enabled: false, available: false },
        },
      },
      llm: {
        availability: "ready",
        agent_enabled: true,
        configured: true,
        provider: "deterministic-e2e",
        model_name: "agent-trust-harness",
      },
    }),
  );
  await page.route("**/api/v1/agent/pins", (route) => json(route, []));
  await page.route("**/api/v1/albums?*", (route) =>
    json(route, {
      albums: [{ album_id: 42, album_name: "Trust Review" }],
      total: 1,
    }),
  );

  let chatRequest: Record<string, unknown> | undefined;
  await page.route(/\/api\/v1\/agent\/chat$/, async (route) => {
    chatRequest = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      headers: { "cache-control": "no-cache" },
      body: sse(
        {
          event: "session_info",
          data: {
            thread_id: "trust-thread",
            run_id: "trust-run-1",
            dropped_mentions: [
              { type: "album", id: "42", label: "Trust Review", reason: "stale_binding" },
            ],
          },
        },
        { event: "run_status", data: { run_id: "trust-run-1", status: "running" } },
        { event: "message", data: { output: "I prepared a safe album update." } },
        {
          event: "action",
          data: {
            action: {
              interrupted: {
                InterruptContexts: [
                  {
                    ID: "trust-interrupt",
                    IsRootCause: true,
                    Info: {
                      effect_id: "trust-effect",
                      action: "add_to_album",
                      count: 2,
                      message: "Add 2 photos to Trust Review?",
                    },
                  },
                ],
              },
            },
          },
        },
        {
          event: "run_status",
          data: { run_id: "trust-run-1", status: "awaiting_confirmation" },
        },
        { event: "done", data: {} },
      ),
    });
  });

  let releaseResume: (() => void) | undefined;
  const resumeGate = new Promise<void>((resolve) => {
    releaseResume = resolve;
  });
  let markResumeStarted: (() => void) | undefined;
  const resumeStarted = new Promise<void>((resolve) => {
    markResumeStarted = resolve;
  });
  await page.route(/\/api\/v1\/agent\/chat\/resume$/, async (route) => {
    markResumeStarted?.();
    await resumeGate;
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      headers: { "cache-control": "no-cache" },
      body: sse(
        {
          event: "session_info",
          data: { thread_id: "trust-thread", run_id: "trust-run-2", dropped_mentions: [] },
        },
        { event: "run_status", data: { run_id: "trust-run-2", status: "running" } },
        {
          event: "side_event",
          data: {
            type: "effect_receipt",
            timestamp: 1,
            tool: { name: "add_to_album", executionId: "trust-effect" },
            execution: { status: "success" },
            receipt: {
              effect_id: "trust-effect",
              tool_name: "add_to_album",
              status: "committed",
              count: 2,
              message: "Added 2 photos to Trust Review.",
            },
          },
        },
        { event: "run_status", data: { run_id: "trust-run-2", status: "completed" } },
        { event: "done", data: {} },
      ),
    });
  });

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/lumilio");
  await expect(page.getByText(t("lumilio.dock.title"), { exact: true })).toBeVisible();

  await page
    .getByRole("button", {
      name: new RegExp(`^${t("lumilio.quickActions.organize.label")}\\b`),
    })
    .click();
  await page.getByTitle(t("lumilio.mention.mention")).click();
  await page.getByRole("option", { name: t("lumilio.mention.album"), exact: true }).click();
  await page.getByRole("option", { name: "Trust Review", exact: true }).click();

  await page.locator("textarea").fill("Organize these safely");
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();

  await expect.poll(() => chatRequest?.mode).toBe("organize");
  expect(chatRequest?.mentions).toEqual([
    { type: "album", id: "42", label: "Trust Review" },
  ]);
  await expect(page.getByText(t("lumilio.quickActions.organize.label"), { exact: true })).toBeVisible();
  await expect(page.getByText("Trust Review", { exact: true })).toBeVisible();
  await expect(page.getByText(t("lumilio.chat.confirmation.title"), { exact: true })).toBeVisible();

  await page
    .getByRole("button", { name: t("lumilio.chat.confirmation.confirm"), exact: true })
    .click();
  await resumeStarted;
  await expect(
    page.getByText(t("lumilio.chat.confirmation.applying"), { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Added 2 photos to Trust Review.", { exact: true })).toHaveCount(0);

  releaseResume?.();
  await expect(page.getByText("Added 2 photos to Trust Review.", { exact: true })).toBeVisible();
});
