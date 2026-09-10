import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { MusicAgentResult } from "./MusicAgentResult";
import { MusicPlayerProvider, useMusicPlayer } from "../state/MusicPlayerProvider";
function QueueProbe() {
  const player = useMusicPlayer();
  return (
    <output aria-label="Preview queue">
      {player.queue.map((item) => item.track.track_id).join(",")}
    </output>
  );
}
describe("music Agent selection", () => {
  it("stays silent until clicked and attaches the edited order when saving", async () => {
    worker.use(
      http.get("*/api/v1/assets/:id/audio/web", () => new HttpResponse(null, { status: 204 })),
      http.get("*/api/v1/agent/refs/selection/music", () =>
        HttpResponse.json({
          tracks: [
            { track_id: "a", title: "Alpha" },
            { track_id: "b", title: "Beta" },
            { track_id: "c", title: "Gamma" },
          ],
          total: 3,
          truncated: false,
        }),
      ),
    );
    const send = vi.fn();
    const screen = await renderWithProviders(
      <MusicPlayerProvider>
        <MusicAgentResult refId="selection" threadId="thread" onSend={send} />
        <QueueProbe />
      </MusicPlayerProvider>,
    );
    await expect
      .element(
        screen.getByRole("checkbox", {
          name: t("music.agent.selectTrack", { title: "Alpha" }),
          exact: true,
        }),
      )
      .toBeVisible();
    await expect
      .element(screen.getByLabelText("Preview queue", { exact: true }))
      .toHaveTextContent("");
    await screen
      .getByRole("checkbox", { name: t("music.agent.selectTrack", { title: "Beta" }), exact: true })
      .click();
    await screen
      .getByRole("button", { name: t("music.agent.moveUp", { title: "Gamma" }), exact: true })
      .click();
    await screen
      .getByRole("button", { name: t("music.agent.moveUp", { title: "Gamma" }), exact: true })
      .click();
    await screen.getByRole("button", { name: t("music.agent.audition"), exact: true }).click();
    await expect
      .element(screen.getByLabelText("Preview queue", { exact: true }))
      .toHaveTextContent("c,a");
    await screen
      .getByRole("textbox", { name: t("music.agent.playlistTitle"), exact: true })
      .fill("My mix");
    await screen.getByRole("button", { name: t("music.agent.save"), exact: true }).click();
    expect(send).toHaveBeenCalledWith(t("music.agent.savePrompt", { title: "My mix" }), ["c", "a"]);
  });
});
