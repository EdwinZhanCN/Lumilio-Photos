import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import AiTab from "./AiTab";

type SystemSettingsDTO = components["schemas"]["dto.SystemSettingsDTO"];

const settingsResponse = {
  llm: {
    agent_enabled: false,
    provider: "",
    model_name: "",
    base_url: "",
    api_key_configured: false,
    supported_providers: [
      { id: "ark", api_key_required: true, base_url_required: false },
      { id: "openai", api_key_required: true, base_url_required: false },
      { id: "deepseek", api_key_required: true, base_url_required: true },
      { id: "ollama", api_key_required: false, base_url_required: true },
      { id: "claude", api_key_required: true, base_url_required: false },
      { id: "gemini", api_key_required: true, base_url_required: false },
      { id: "qwen", api_key_required: true, base_url_required: true },
      { id: "openrouter", api_key_required: true, base_url_required: false },
      { id: "future-provider", api_key_required: true, base_url_required: false },
    ],
  },
  ml: {
    semantic_enabled: false,
    video_semantic_enabled: false,
    bioclip_enabled: false,
    ocr_enabled: false,
    face_enabled: false,
  },
  backup: { enabled: false, interval_hours: 24, keep_last: 14 },
  geocoding: {
    provider: "disabled",
    nominatim_endpoint: "https://nominatim.openstreetmap.org/reverse",
    language: "en",
    user_agent: "Lumilio-Photos/1.0",
  },
  updated_at: "2026-08-18T00:00:00Z",
} satisfies SystemSettingsDTO;

describe("AiTab provider contract", () => {
  it("renders server-advertised providers and validates Qwen with its requirements", async () => {
    let validationBody: unknown;
    worker.use(
      http.get("*/api/v1/settings/system", () => HttpResponse.json(settingsResponse)),
      http.post("*/api/v1/settings/system/validate-llm", async ({ request }) => {
        validationBody = await request.json();
        return HttpResponse.json({ valid: true });
      }),
    );

    const screen = await renderWithProviders(<AiTab />);

    const providerControl = screen.getByRole("button", {
      name: t("settings.aiSettings.provider"),
    });
    for (const label of [
      "Ark",
      "OpenAI",
      "DeepSeek",
      "Ollama",
      "Claude",
      "Gemini",
      "Qwen",
      "OpenRouter",
    ]) {
      await providerControl.click();
      await screen.getByRole("button", { name: label }).click();
      await expect.element(providerControl).toHaveTextContent(label);
    }
    await providerControl.click();
    await expect
      .element(screen.getByRole("button", { name: "future-provider" }))
      .not.toBeInTheDocument();
    await screen.getByRole("button", { name: "Qwen" }).click();
    await screen.getByLabelText(t("settings.aiSettings.modelName")).fill("qwen-plus");
    await screen
      .getByLabelText(t("settings.aiSettings.baseUrl"))
      .fill("https://dashscope.example/v1");
    await screen
      .getByRole("textbox", { name: t("settings.aiSettings.apiKey"), exact: true })
      .fill("draft-secret");
    await screen.getByRole("button", { name: t("settings.aiSettings.validate") }).click();

    await vi.waitFor(() => {
      expect(validationBody).toEqual({
        provider: "qwen",
        model_name: "qwen-plus",
        base_url: "https://dashscope.example/v1",
        api_key: "draft-secret",
        use_stored_api_key: false,
      });
    });
    await expect
      .element(screen.getByText(t("settings.aiSettings.validationSuccess")))
      .toBeVisible();
  });
});
