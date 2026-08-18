import { describe, expect, it } from "vite-plus/test";
import { LLM_PROVIDER_IDS, normalizeProvider, normalizeProviderDescriptors } from "./llmProviders";

describe("LLM provider model", () => {
  it("keeps the expected product-supported provider IDs in stable order", () => {
    expect(LLM_PROVIDER_IDS).toEqual([
      "ark",
      "openai",
      "deepseek",
      "ollama",
      "claude",
      "gemini",
      "qwen",
      "openrouter",
    ]);
  });

  it("normalizes the server contract in its advertised order", () => {
    expect(
      normalizeProviderDescriptors([
        { id: "qwen", api_key_required: true, base_url_required: true },
        { id: "ollama", api_key_required: false, base_url_required: true },
        { id: "qwen", api_key_required: false, base_url_required: false },
        { id: "future-provider", api_key_required: true },
      ]),
    ).toEqual([
      { id: "qwen", apiKeyRequired: true, baseURLRequired: true },
      { id: "ollama", apiKeyRequired: false, baseURLRequired: true },
    ]);
  });

  it("does not silently select an unknown provider", () => {
    expect(normalizeProvider("future-provider")).toBe("none");
    expect(normalizeProvider("claude")).toBe("claude");
  });
});
