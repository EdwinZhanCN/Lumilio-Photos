import { describe, expect, it, vi } from "vite-plus/test";
import { Routes, Route } from "react-router-dom";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { MusicPlayerProvider } from "../../state/MusicPlayerProvider";
import MusicArtistDetailsFlow from "./MusicArtistDetailsFlow";

describe("artist track sorting", () => {
  it("requests globally sorted pages from the API", async () => {
    const requests: string[] = [];
    worker.use(
      http.get("*/api/v1/music/artists/artist", () =>
        HttpResponse.json({ artist_id: "artist", display_name: "Fixture artist", revision: 1 }),
      ),
      http.get("*/api/v1/music/tracks", ({ request }) => {
        requests.push(request.url);
        return HttpResponse.json({ items: [], total: 100 });
      }),
    );
    const screen = await renderWithProviders(
      <MusicPlayerProvider>
        <Routes>
          <Route path="/music/artists/:artistId" element={<MusicArtistDetailsFlow />} />
        </Routes>
      </MusicPlayerProvider>,
      { route: "/music/artists/artist" },
    );
    await screen
      .getByRole("button", {
        name: t("music.tracks.sortBy", { sort: t("music.tracks.sort.recent") }),
        exact: true,
      })
      .click();
    await screen.getByRole("button", { name: t("music.tracks.sort.title"), exact: true }).click();
    await vi.waitFor(() =>
      expect(requests.some((url) => new URL(url).searchParams.get("sort") === "title")).toBe(true),
    );
    await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
    await vi.waitFor(() =>
      expect(
        requests.some((url) => {
          const params = new URL(url).searchParams;
          return (
            params.get("sort") === "title" &&
            params.get("offset") === "50" &&
            params.get("artist_id") === "artist"
          );
        }),
      ).toBe(true),
    );
  });
});
