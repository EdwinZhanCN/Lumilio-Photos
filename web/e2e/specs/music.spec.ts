import { readFileSync } from "node:fs";
import path from "node:path";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api, baseURL } from "../support/api";
import { smokeAsset } from "../support/assets";
import { t } from "../support/i18n";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type TrackPage = components["schemas"]["dto.MusicTrackPageDTO"];
type Album = components["schemas"]["dto.MusicAlbumDTO"];
type Upload = components["schemas"]["dto.UploadResponseDTO"];
type Operations = components["schemas"]["dto.UploadOperationStatusResponseDTO"];

// Original Bandcamp files shared with demo: AAC remaster and original FLAC.
const sources = [
  "bandcamp-khoidauminh-remastered-01",
  "bandcamp-khoidauminh-remastered-02",
  "bandcamp-khoidauminh-chamber-01",
] as const;

test("@smoke music imports tags, separates editions, and preserves album playback across navigation", async ({
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
  const remaster = tracks.filter(
    (track) => track.album_title === "Chambers (remastered) (public domain)",
  );
  expect(remaster).toHaveLength(2);
  const first = remaster.find((track) => track.track_number === 1)!;
  const second = remaster.find((track) => track.track_number === 2)!;
  expect(first).toMatchObject({ title: "Dark chamber", artist_name: "khoidauminh" });
  expect(second).toMatchObject({ title: "Light chamber" });
  expect(first.duration).toBeGreaterThan(190);
  const original = tracks.find((track) => track.album_title === "Chamber")!;
  expect(original).toMatchObject({ title: "Dark Chamber", track_number: 1 });
  // These originals have no stable release IDs; group explicitly through the
  // public API instead of making title-only automatic grouping a requirement.
  for (const members of [remaster, [original]]) {
    const created = await api<Album>("/api/v1/music/albums", {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({
        title: members[0].album_title,
        artist_names: [members[0].artist_name],
      }),
    });
    expect(created.album_id).toBeTruthy();
    for (const track of members) {
      await api(`/api/v1/music/tracks/${track.track_id}/album`, {
        method: "PUT",
        token: workspace.token,
        body: JSON.stringify({ album_id: created.album_id, revision: track.revision }),
      });
      track.album_id = created.album_id;
    }
  }
  expect(original.album_id).not.toBe(first.album_id);
  const album = await api<Album>(`/api/v1/music/albums/${first.album_id}`, {
    token: workspace.token,
  });
  expect(album.tracks?.map((track) => track.track_id)).toEqual([first.track_id, second.track_id]);

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto(`/music/albums/${first.album_id}`);
  await expect(page.getByRole("heading", { name: album.title!, exact: true })).toBeVisible();
  await page.getByRole("button", { name: t("music.actions.playAlbum"), exact: true }).click();
  const dock = page.getByRole("region", { name: t("music.player.label"), exact: true });
  await expect(dock.getByText(first.title!, { exact: true })).toBeVisible();
  const audio = page.locator("audio");
  await expect
    .poll(() =>
      audio.evaluate(
        (element: HTMLAudioElement) =>
          !element.paused && element.currentTime > 0 && element.error === null,
      ),
    )
    .toBe(true);
  const source = await audio.getAttribute("src");
  const position = await audio.evaluate((element: HTMLAudioElement) => element.currentTime);
  // Follow the real SPA link: a full page load would mask a provider remount.
  await page
    .getByRole("navigation", { name: "Breadcrumb" })
    .getByRole("link", { name: t("music.title"), exact: true })
    .click();
  await expect(page).toHaveURL(/\/music$/);
  await expect(audio).toHaveAttribute("src", source!);
  await expect
    .poll(() => audio.evaluate((element: HTMLAudioElement) => element.currentTime))
    .toBeGreaterThan(position);
  await dock.getByRole("button", { name: t("music.player.next"), exact: true }).click();
  await expect(dock.getByText(second.title!, { exact: true })).toBeVisible();
  await expect(audio).not.toHaveAttribute("src", source!);
  await expect
    .poll(() =>
      audio.evaluate(
        (element: HTMLAudioElement) =>
          !element.paused && element.currentTime > 0 && element.error === null,
      ),
    )
    .toBe(true);
  await dock.getByRole("button", { name: t("music.queue.title"), exact: true }).click();
  await page.getByRole("button", { name: t("music.player.close"), exact: true }).click();
  await expect(dock).toHaveCount(0);
  await expect
    .poll(() =>
      audio.evaluate((element: HTMLAudioElement) => element.paused && !element.getAttribute("src")),
    )
    .toBe(true);
});
