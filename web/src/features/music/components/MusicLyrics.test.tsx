import { describe, expect, it, vi } from "vite-plus/test";
import { useQueryClient } from "@tanstack/react-query";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import MusicLyrics from "./MusicLyrics";

function RefreshLyrics() {
  const queryClient = useQueryClient();
  return (
    <button
      onClick={() =>
        void queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/music/tracks/{id}/lyrics"],
        })
      }
    >
      Refresh fixture
    </button>
  );
}
describe("MusicLyrics", () => {
  it("highlights the current line and seeks using its timestamp", async () => {
    worker.use(
      http.get("*/api/v1/music/tracks/timed/lyrics", () =>
        HttpResponse.json({ content: "[00:01]Opening\n[00:04]Chorus", revision: 1 }),
      ),
    );
    const seek = vi.fn();
    const screen = await renderWithProviders(
      <MusicLyrics trackId="timed" currentTime={2} onSeek={seek} />,
    );
    await expect
      .element(screen.getByRole("button", { name: "Opening", exact: true }))
      .toHaveAttribute("aria-current", "true");
    await screen.getByRole("button", { name: "Chorus", exact: true }).click();
    expect(seek).toHaveBeenCalledWith(4);
  });
  it("preserves the draft and its revision through a background refetch", async () => {
    let revision = 1;
    let reads = 0;
    let saved: unknown;
    worker.use(
      http.get("*/api/v1/music/tracks/edited/lyrics", () => {
        reads++;
        return HttpResponse.json({ content: `Saved ${revision}`, revision });
      }),
      http.put("*/api/v1/music/tracks/edited/lyrics", async ({ request }) => {
        saved = await request.json();
        return HttpResponse.json(
          { type: "about:blank", title: "Fixture error", status: 409 },
          { status: 409, headers: { "Content-Type": "application/problem+json" } },
        );
      }),
    );
    const screen = await renderWithProviders(
      <>
        <RefreshLyrics />
        <MusicLyrics trackId="edited" editable />
      </>,
    );
    const editor = screen.getByRole("textbox", { name: t("music.lyrics.title"), exact: true });
    await expect.element(editor).toHaveValue("Saved 1");
    await editor.fill("Unsaved local draft");
    revision = 2;
    await screen.getByRole("button", { name: "Refresh fixture", exact: true }).click();
    await vi.waitFor(() => expect(reads).toBeGreaterThan(1));
    await expect.element(editor).toHaveValue("Unsaved local draft");
    await screen.getByRole("button", { name: t("music.actions.save"), exact: true }).click();
    await expect.element(screen.getByRole("alert")).toBeVisible();
    expect(saved).toEqual({ content: "Unsaved local draft", revision: 1 });
    await expect.element(editor).toHaveValue("Unsaved local draft");
  });
});
