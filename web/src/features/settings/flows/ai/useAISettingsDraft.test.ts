import { describe, expect, it } from "vite-plus/test";
import { buildValidationPayload, type AISettingsDraft } from "./useAISettingsDraft";

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
    expect(buildValidationPayload(draft(), true, "openai")).toMatchObject({
      provider: "openai",
      use_stored_api_key: true,
    });
    expect(() => buildValidationPayload(draft({ provider: "deepseek" }), true, "openai")).toThrow(
      /API key/,
    );
  });

  it("sends an unsaved replacement secret without persisting it", () => {
    expect(buildValidationPayload(draft({ apiKey: "new-secret" }), true, "openai")).toMatchObject({
      api_key: "new-secret",
      use_stored_api_key: false,
    });
  });

  it("allows local Ollama validation without an API key", () => {
    expect(
      buildValidationPayload(
        draft({ provider: "ollama", modelName: "qwen3", baseURL: "http://localhost:11434" }),
        false,
        "none",
      ),
    ).toMatchObject({ provider: "ollama", use_stored_api_key: false });
  });

  it("rejects an unset provider before making a request", () => {
    expect(() => buildValidationPayload(draft({ provider: "none" }), false, "none")).toThrow(
      /Select an LLM provider/,
    );
  });
  it("requires an explicit endpoint for DeepSeek", () => {
    expect(() =>
      buildValidationPayload(
        draft({ provider: "deepseek", modelName: "deepseek-chat", baseURL: "" }),
        false,
        "none",
      ),
    ).toThrow(/base URL/);
  });

});
