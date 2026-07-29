import { readFileSync } from "node:fs";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import { t } from "../support/i18n";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type EventList = components["schemas"]["dto.EventListPageDTO"];
type EventAssets = components["schemas"]["dto.EventAssetsPageDTO"];
type EventMutation = components["schemas"]["dto.EventMutationResponseDTO"];
type EventShare = components["schemas"]["dto.CreateShareLinkResponseDTO"];
type PublicShare = components["schemas"]["dto.PublicShareMetadataDTO"];

function uniqueJpeg(sourcePath: string, markerText: string) {
  const source = readFileSync(sourcePath);
  const endOfImage = source.lastIndexOf(Buffer.from([0xff, 0xd9]));
  if (endOfImage < 0) throw new Error("Events fixture is not a JPEG");
  const marker = Buffer.from(markerText, "utf8");
  const markerLength = marker.length + 2;
  return Buffer.concat([
    source.subarray(0, endOfImage),
    Buffer.from([0xff, 0xfe, markerLength >> 8, markerLength & 0xff]),
    marker,
    source.subarray(endOfImage),
  ]);
}

test("@smoke Events rebuild, correct, redirect, and freeze a share snapshot", async ({
  page,
  workspace,
}) => {
  test.setTimeout(120_000);
  await new LoginPage(page).signIn(workspace.username, workspace.password);

  await page.goto("/manage");
  const eventFilenames = [
    `e2e-event-${workspace.username}-1.jpg`,
    `e2e-event-${workspace.username}-2.jpg`,
  ];
  await page
    .getByLabel(t("upload.UnifiedUploadSection.upload_target_label"))
    .selectOption({ label: workspace.repositoryName });
  await page.locator('input[type="file"]').setInputFiles(
    eventFilenames.map((name) => ({
      name,
      mimeType: "image/jpeg",
      buffer: uniqueJpeg(workspace.authIsolationSource, `lumilio-events:${name}`),
    })),
  );
  const accepted = page.waitForResponse(
    (response) =>
      /\/api\/v1\/assets(\/batch)?$/.test(new URL(response.url()).pathname) &&
      response.request().method() === "POST" &&
      response.ok(),
  );
  await page
    .getByRole("button", {
      name: t("upload.UnifiedUploadSection.upload_button", { countLabel: " (2)" }),
    })
    .click();
  await accepted;

  for (const filename of eventFilenames) {
    await expect(async () => {
      const result = await api<components["schemas"]["dto.QueryAssetsResponseDTO"]>(
        "/api/v1/assets/list",
        {
          method: "POST",
          token: workspace.token,
          body: JSON.stringify({
            query: filename,
            search_type: "filename",
            filter: { repository_id: workspace.repositoryId },
            pagination: { limit: 10, offset: 0 },
            stack_mode: "expanded",
          }),
        },
      );
      expect(result.items?.length).toBe(1);
    }).toPass({ timeout: 60_000 });
  }

  await api("/api/v1/events/rebuild", {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({ dry_run: false }),
  });
  let eventList = await api<EventList>("/api/v1/events?limit=100", {
    token: workspace.token,
  });
  expect(eventList.events?.length).toBeGreaterThan(0);

  let survivor = eventList.events?.[0]?.event_id;
  if (!survivor) throw new Error("Event rebuild did not return an Event ID");
  if ((eventList.events?.length ?? 0) > 1) {
    const other = eventList.events?.[1]?.event_id;
    if (!other) throw new Error("second Event has no ID");
    await api<EventMutation>("/api/v1/events/merge", {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({ event_ids: [survivor, other], survivor_event_id: survivor }),
    });
    const redirected = await api<components["schemas"]["dto.EventDetailDTO"]>(
      `/api/v1/events/${other}`,
      { token: workspace.token },
    );
    expect(redirected.event_id).toBe(survivor);
    expect(redirected.redirected_from).toBe(other);
  }

  let members = await api<EventAssets>(`/api/v1/events/${survivor}/assets?limit=500`, {
    token: workspace.token,
  });
  expect(members.assets?.length).toBeGreaterThanOrEqual(2);
  const splitBefore = members.assets?.[1]?.media_item_id;
  if (!splitBefore) throw new Error("second Event member has no logical media ID");
  await api<EventMutation>(`/api/v1/events/${survivor}/split`, {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({ before_media_item_id: splitBefore }),
  });
  eventList = await api<EventList>("/api/v1/events?limit=100", { token: workspace.token });
  const splitEvent = eventList.events?.find((item) => item.event_id !== survivor)?.event_id;
  if (!splitEvent) throw new Error("split did not create a second Event");
  await api<EventMutation>("/api/v1/events/merge", {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({
      event_ids: [survivor, splitEvent],
      survivor_event_id: survivor,
    }),
  });

  const title = `E2E Event ${workspace.username}`;
  await api<EventMutation>(`/api/v1/events/${survivor}`, {
    method: "PATCH",
    token: workspace.token,
    body: JSON.stringify({ title_override: title }),
  });
  await page.goto(`/collections/events/${survivor}`);
  await expect(page.getByRole("heading", { name: title }).first()).toBeVisible();

  const share = await api<EventShare>(`/api/v1/events/${survivor}/share`, {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({ title, allow_download: false, include_originals: false }),
  });
  if (!share.token) throw new Error("Event share did not return a token");
  const before = await api<PublicShare>(`/api/v1/public/shares/${share.token}`);
  members = await api<EventAssets>(`/api/v1/events/${survivor}/assets?limit=500`, {
    token: workspace.token,
  });
  const removable = members.assets?.at(-1)?.media_item_id;
  if (!removable) throw new Error("Event has no removable member");
  await api<EventMutation>(`/api/v1/events/${survivor}/members/${removable}`, {
    method: "DELETE",
    token: workspace.token,
  });
  const after = await api<PublicShare>(`/api/v1/public/shares/${share.token}`);
  expect(after.asset_count).toBe(before.asset_count);
});
