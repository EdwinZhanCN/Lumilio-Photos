import type { components } from "@/lib/http-commons/schema.d.ts";
import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import MusicRating from "./MusicRating";

describe("MusicRating", () => {
  it("re-reads the saved rating and supports clearing it", async () => {
    let rating = 2;
    worker.use(
      http.get("*/api/v1/music/tracks/rated", () =>
        HttpResponse.json({ track_id: "rated", rating }),
      ),
      http.put("*/api/v1/assets/rated/rating", async ({ request }) => {
        const body = (await request.json()) as components["schemas"]["dto.UpdateRatingRequestDTO"];
        rating = body.rating ?? 0;
        return HttpResponse.json({ message: "saved" });
      }),
    );
    const screen = await renderWithProviders(
      <MusicRating track={{ track_id: "rated", rating: 2 }} />,
    );
    const four = screen.getByRole("button", {
      name: t("music.rating.stars", { count: 4 }),
      exact: true,
    });
    await four.click();
    await expect.element(four).toHaveAttribute("aria-pressed", "true");
    expect(rating).toBe(4);
    await screen.getByRole("button", { name: t("music.rating.clear"), exact: true }).click();
    await expect.element(four).toHaveAttribute("aria-pressed", "false");
    expect(rating).toBe(0);
  });
  it("retains the confirmed value when saving fails", async () => {
    worker.use(
      http.put("*/api/v1/assets/rated/rating", () =>
        HttpResponse.json(
          { type: "about:blank", title: "Fixture error", status: 503 },
          { status: 503, headers: { "Content-Type": "application/problem+json" } },
        ),
      ),
    );
    const screen = await renderWithProviders(
      <MusicRating track={{ track_id: "rated", rating: 2 }} />,
    );
    await screen
      .getByRole("button", { name: t("music.rating.stars", { count: 5 }), exact: true })
      .click();
    await expect.element(screen.getByRole("alert")).toBeVisible();
    await expect
      .element(
        screen.getByRole("button", { name: t("music.rating.stars", { count: 2 }), exact: true }),
      )
      .toHaveAttribute("aria-pressed", "true");
  });
});
