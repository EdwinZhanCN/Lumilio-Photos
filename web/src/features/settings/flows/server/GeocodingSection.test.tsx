import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import GeocodingSection from "./GeocodingSection";

const initialSettings = {
  llm: { agent_enabled: false, provider: "", model_name: "", api_key_configured: false },
  ml: {},
  backup: { enabled: true, interval_hours: 24, keep_last: 14 },
  geocoding: {
    provider: "disabled" as const,
    nominatim_endpoint: "https://nominatim.openstreetmap.org/reverse",
    language: "en",
    user_agent: "Lumilio-Photos/1.0",
  },
  updated_at: "2026-08-13T12:00:00Z",
};

describe("GeocodingSection", () => {
  it("hydrates a local draft and sends the nested aggregate only on Save", async () => {
    let current = initialSettings;
    let patchBody: unknown;
    let patchCount = 0;
    worker.use(
      http.get("*/api/v1/settings/system", () => HttpResponse.json(current)),
      http.patch("*/api/v1/settings/system", async ({ request }) => {
        patchCount += 1;
        patchBody = await request.json();
        const body = patchBody as { geocoding: typeof current.geocoding };
        current = { ...current, geocoding: body.geocoding };
        return HttpResponse.json(current);
      }),
    );

    const screen = await renderWithProviders(<GeocodingSection />);
    const endpoint = screen.getByLabelText(t("settings.serverSettings.geocoding.endpointLabel"));
    const userAgent = screen.getByLabelText(t("settings.serverSettings.geocoding.userAgentLabel"));
    await endpoint.fill("http://127.0.0.1:8080/reverse");
    await userAgent.fill("Lumilio-Test/1.0");

    expect(patchCount).toBe(0);
    await screen.getByRole("button", { name: t("settings.section.save"), exact: true }).click();
    await expect
      .element(screen.getByText(t("settings.section.saved"), { exact: true }))
      .toBeVisible();

    expect(patchBody).toEqual({
      geocoding: {
        provider: "disabled",
        nominatim_endpoint: "http://127.0.0.1:8080/reverse",
        language: "en",
        user_agent: "Lumilio-Test/1.0",
      },
    });
  });

  it("keeps the draft local when loading the server settings fails", async () => {
    worker.use(
      http.get("*/api/v1/settings/system", () =>
        HttpResponse.json({ message: "settings unavailable" }, { status: 500 }),
      ),
    );

    const screen = await renderWithProviders(<GeocodingSection />);
    await expect
      .element(screen.getByText(t("settings.serverSettings.geocoding.loadError"), { exact: true }))
      .toBeVisible();
  });
});
