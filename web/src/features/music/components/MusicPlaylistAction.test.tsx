import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import MusicPlaylistAction from "./MusicPlaylistAction";

describe("MusicPlaylistAction", () => {
  it("closes after creating and clears the title when reopened", async () => {
    let saved: unknown;
    worker.use(
      http.post("*/api/v1/music/playlists", async ({ request }) => {
        saved = await request.json();
        return HttpResponse.json({ playlist_id: "new-playlist", title: "Evening", revision: 1 });
      }),
    );
    const screen = await renderWithProviders(<MusicPlaylistAction />);
    await screen.getByRole("button", { name: t("music.playlists.create"), exact: true }).click();
    await screen.getByRole("textbox", { name: t("music.fields.playlistTitle") }).fill("Evening");
    await screen
      .getByRole("dialog")
      .getByRole("button", { name: t("music.playlists.create"), exact: true })
      .click();
    await vi.waitFor(() => expect(saved).toEqual({ title: "Evening" }));
    await vi.waitFor(() => expect(document.querySelector("dialog")?.open).toBe(false));
    await screen.getByRole("button", { name: t("music.playlists.create"), exact: true }).click();
    await expect
      .element(screen.getByRole("textbox", { name: t("music.fields.playlistTitle") }))
      .toHaveValue("");
  });

  it("reuses a created playlist when adding its initial track fails", async () => {
    let creates = 0;
    let adds = 0;
    worker.use(
      http.get("*/api/v1/music/playlists", () => HttpResponse.json({ items: [], total: 0 })),
      http.post("*/api/v1/music/playlists", () => {
        creates++;
        return HttpResponse.json({ playlist_id: "new-playlist", title: "Evening", revision: 1 });
      }),
      http.post("*/api/v1/music/playlists/new-playlist/entries", () => {
        adds++;
        return adds === 1
          ? HttpResponse.json({ error: "temporary" }, { status: 503 })
          : HttpResponse.json({ entry_id: "entry-1" });
      }),
    );
    const screen = await renderWithProviders(<MusicPlaylistAction trackId="track-1" />);
    await screen.getByRole("button", { name: t("music.playlists.addLabel"), exact: true }).click();
    await screen.getByRole("button", { name: t("music.playlists.create"), exact: true }).click();
    await screen.getByRole("textbox", { name: t("music.fields.playlistTitle") }).fill("Evening");
    await screen.getByRole("button", { name: t("music.playlists.create"), exact: true }).click();
    await expect.element(screen.getByRole("alert")).toBeVisible();
    await screen.getByRole("button", { name: t("music.playlists.create"), exact: true }).click();
    await vi.waitFor(() => expect(document.querySelector("dialog")?.open).toBe(false));
    expect(creates).toBe(1);
    expect(adds).toBe(2);
  });
});
