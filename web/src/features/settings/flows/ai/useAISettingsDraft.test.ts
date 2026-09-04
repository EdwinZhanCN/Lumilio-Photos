import { describe, expect, it } from "vite-plus/test";
import { buildValidationPayload, type AISettingsDraft } from "./useAISettingsDraft";
import type { LLMProviderDescriptor } from "../../model/llmProviders";

const providers: LLMProviderDescriptor[] = [
  { id: "ark", apiKeyRequired: true, baseURLRequired: false },
  { id: "openai", apiKeyRequired: true, baseURLRequired: false },
  { id: "deepseek", apiKeyRequired: true, baseURLRequired: false },
  { id: "ollama", apiKeyRequired: false, baseURLRequired: true },
  { id: "claude", apiKeyRequired: true, baseURLRequired: false },
  { id: "gemini", apiKeyRequired: true, baseURLRequired: false },
  { id: "qwen", apiKeyRequired: true, baseURLRequired: true },
  { id: "openrouter", apiKeyRequired: true, baseURLRequired: false },
];

const draft = (overrides: Partial<AISettingsDraft["llm"]> = {}): AISettingsDraft => ({
  llm: {
    agentEnabled: true,
    provider: "openai",
    modelName: "gpt-4.1-mini",
    baseURL: "https://api.openai.com/v1",
    apiKey: "",
    clearStoredKey: false,
    ...overrides,
  },
  ml: {
    semanticEnabled: true,
    videoSemanticEnabled: true,
    bioclipEnabled: true,
    ocrEnabled: true,
    faceEnabled: true,
  },
});

describe("buildValidationPayload", () => {
  it("reuses a stored key only for the same provider", () => {
    expect(buildValidationPayload(draft(), true, "openai", providers)).toMatchObject({
      provider: "openai",
      use_stored_api_key: true,
    });
    expect(() =>
      buildValidationPayload(
        draft({ provider: "deepseek", baseURL: "https://deepseek.example/v1" }),
        true,
        "openai",
        providers,
      ),
    ).toThrow(/API key/);
  });

  it("sends an unsaved replacement secret without persisting it", () => {
    expect(
      buildValidationPayload(draft({ apiKey: "new-secret" }), true, "openai", providers),
    ).toMatchObject({ api_key: "new-secret", use_stored_api_key: false });
  });

  it("allows local Ollama validation without an API key", () => {
    expect(
      buildValidationPayload(
        draft({ provider: "ollama", modelName: "qwen3", baseURL: "http://localhost:11434" }),
        false,
        "none",
        providers,
      ),
    ).toMatchObject({ provider: "ollama", use_stored_api_key: false });
  });

  it("rejects an unset provider before making a request", () => {
    expect(() =>
      buildValidationPayload(draft({ provider: "none" }), false, "none", providers),
    ).toThrow(/Select an LLM provider/);
  });
  it("lets DeepSeek use its adapter-owned endpoint", () => {
    expect(
      buildValidationPayload(
        draft({
          provider: "deepseek",
          modelName: "deepseek-v4-flash",
          baseURL: "",
          apiKey: "secret",
        }),
        false,
        "none",
        providers,
      ),
    ).toMatchObject({ provider: "deepseek", base_url: "", use_stored_api_key: false });
  });

  it("uses the server descriptor for Qwen requirements", () => {
    expect(() =>
      buildValidationPayload(
        draft({ provider: "qwen", modelName: "qwen-plus", baseURL: "", apiKey: "secret" }),
        false,
        "none",
        providers,
      ),
    ).toThrow(/base URL/);
  });

  it("rejects a provider that the server did not advertise", () => {
    expect(() =>
      buildValidationPayload(
        draft({ provider: "claude", modelName: "claude-sonnet", apiKey: "secret" }),
        false,
        "none",
        providers.filter(({ id }) => id !== "claude"),
      ),
    ).toThrow(/not supported by this server/);
  });
});
