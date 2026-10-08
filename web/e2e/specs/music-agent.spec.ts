import { readFileSync } from "node:fs";
import path from "node:path";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api, baseURL } from "../support/api";
import { PLAYBACK_START_TIMEOUT, smokeAsset } from "../support/assets";
import { t } from "../support/i18n";
import { agentScenarioPrompt } from "../support/agentRuntime";
import type { components } from "../../src/lib/http-commons/schema.d.ts";
type TrackPage = components["schemas"]["dto.MusicTrackPageDTO"];
type Upload = components["schemas"]["dto.UploadResponseDTO"];
type Operations = components["schemas"]["dto.UploadOperationStatusResponseDTO"];
type PlaylistPage = components["schemas"]["dto.MusicPlaylistPageDTO"];
type EntryPage = components["schemas"]["dto.MusicPlaylistEntriesResponseDTO"];
const sources = [
  "bandcamp-khoidauminh-remastered-01",
  "bandcamp-khoidauminh-remastered-02",
  "bandcamp-khoidauminh-chamber-01",
] as const;
test("@agent-runtime music selection auditions, refines and saves after confirmation", async ({
  page,
  request,
  workspace,
}) => {
  test.setTimeout(180_000);
  const receipts: string[] = [];
  for (const id of sources) {
    const file = smokeAsset(id);
    const response = await request.post(`${baseURL}/api/v1/assets`, {
      headers: { authorization: `Bearer ${workspace.token}` },
      multipart: {
        repository_id: workspace.repositoryId,
        file: {
          name: path.basename(file),
          mimeType: file.endsWith(".flac") ? "audio/flac" : "audio/mp4",
          buffer: readFileSync(file),
        },
      },
    });
    expect(response.ok()).toBeTruthy();
    const upload: Upload = await response.json();
    expect(upload.receipt_id).toBeTruthy();
    receipts.push(upload.receipt_id!);
  }
  await expect(async () => {
    const response = await api<Operations>(
      `/api/v1/assets/batch/operations?receipt_ids=${receipts.join(",")}`,
      { token: workspace.token },
    );
    expect(response.operations?.map((operation) => operation.receipt_id).sort()).toEqual(
      [...receipts].sort(),
    );
    expect(response.operations?.every((operation) => operation.terminal && operation.success)).toBe(
      true,
    );
  }).toPass({ timeout: 90_000 });

  let tracks: NonNullable<TrackPage["items"]> = [];
  await expect(async () => {
    const response = await api<TrackPage>("/api/v1/music/tracks?limit=100&sort=track", {
      token: workspace.token,
    });
    tracks = response.items ?? [];
    expect(tracks).toHaveLength(3);
    expect(
      tracks.filter((track) => track.album_title === "Chambers (remastered) (public domain)"),
    ).toHaveLength(2);
    expect(tracks.find((track) => track.album_title === "Chamber")).toBeTruthy();
  }).toPass({ timeout: 90_000 });

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  let hydrationURL = "";
  page.on("response", (response) => {
    const url = new URL(response.url());
    if (
      url.pathname.startsWith("/api/v1/agent/refs/") &&
      url.pathname.endsWith("/music") &&
      response.ok()
    )
      hydrationURL = response.url();
  });
  await page.goto("/lumilio");
  await page.locator("textarea").fill(agentScenarioPrompt({ name: "music" }));
  await page.getByRole("button", { name: t("lumilio.input.send"), exact: true }).click();
  const selection = page.getByRole("region", { name: t("music.agent.selection"), exact: true });
  await expect(selection).toHaveCount(1);
  await expect(selection.getByRole("checkbox")).toHaveCount(3);
  const wrongThread = new URL(hydrationURL);
  wrongThread.searchParams.set("thread_id", "unrelated-thread");
  const denied = await request.get(wrongThread.toString(), {
    headers: { authorization: `Bearer ${workspace.token}` },
  });
  expect(denied.status()).toBe(404);
  await expect
    .poll(() =>
      page
        .locator("audio")
        .evaluateAll((nodes) =>
          nodes.every((node) => node instanceof HTMLAudioElement && node.paused),
        ),
    )
    .toBe(true);
  await selection.getByRole("button", { name: t("music.agent.audition"), exact: true }).click();
  await expect
    .poll(
      () =>
        page
          .locator("audio")
          .evaluateAll((nodes) =>
            nodes.some(
              (node) => node instanceof HTMLAudioElement && !node.paused && node.currentTime > 0,
            ),
          ),
      { timeout: PLAYBACK_START_TIMEOUT },
    )
    .toBe(true);
  const light = tracks.find((track) => track.title === "Light chamber")!;
  await selection
    .getByRole("button", { name: t("music.agent.moveUp", { title: "Light chamber" }), exact: true })
    .click();
  await selection
    .getByRole("button", { name: t("music.agent.moveUp", { title: "Light chamber" }), exact: true })
    .click();
  await expect(selection.getByRole("listitem").first()).toContainText("Light chamber");
  await selection
    .getByRole("checkbox", {
      name: t("music.agent.selectTrack", { title: "Dark Chamber" }),
      exact: true,
    })
    .uncheck();
  await selection
    .getByRole("textbox", { name: t("music.agent.refine"), exact: true })
    .fill("Keep only Light");
  await selection.getByRole("button", { name: t("music.agent.refine"), exact: true }).click();
  await expect(selection).toHaveCount(2);
  const refined = selection.last();
  await expect(refined.getByRole("checkbox")).toHaveCount(1);
  await expect(refined).toContainText("Light chamber");
  const title = "Agent ordered mix";
  await refined
    .getByRole("textbox", { name: t("music.agent.playlistTitle"), exact: true })
    .fill(title);
  await refined.getByRole("button", { name: t("music.agent.save"), exact: true }).click();
  await expect(
    page.getByText(t("music.agent.confirmCreate_one", { title, count: 1 }), { exact: true }),
  ).toBeVisible();
  const before = await api<PlaylistPage>("/api/v1/music/playlists", { token: workspace.token });
  expect(before.items?.find((playlist) => playlist.title === title)).toBeUndefined();
  await page
    .getByRole("button", { name: t("lumilio.chat.confirmation.confirm"), exact: true })
    .click();
  const open = page.getByRole("link", { name: t("music.agent.openPlaylist"), exact: true });
  await expect(open).toBeVisible();
  const after = await api<PlaylistPage>("/api/v1/music/playlists", { token: workspace.token });
  const saved = after.items?.filter((playlist) => playlist.title === title) ?? [];
  expect(saved).toHaveLength(1);
  const entries = await api<EntryPage>(`/api/v1/music/playlists/${saved[0].playlist_id}/entries`, {
    token: workspace.token,
  });
  expect(entries.items?.map((entry) => entry.track_id)).toEqual([light.track_id]);
  await open.click();
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  await expect
    .poll(
      () =>
        page
          .locator("audio")
          .evaluateAll((nodes) =>
            nodes.some(
              (node) => node instanceof HTMLAudioElement && !node.paused && node.currentTime > 0,
            ),
          ),
      { timeout: PLAYBACK_START_TIMEOUT },
    )
    .toBe(true);
});
