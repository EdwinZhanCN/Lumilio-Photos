import type { Schemas } from "../api/types";

export const LLM_PROVIDER_IDS = [
  "ark",
  "openai",
  "deepseek",
  "ollama",
  "claude",
  "gemini",
  "qwen",
  "openrouter",
] as const;

export type LLMProvider = (typeof LLM_PROVIDER_IDS)[number];
export type AgentProvider = "none" | LLMProvider;

export interface LLMProviderDescriptor {
  id: LLMProvider;
  apiKeyRequired: boolean;
  baseURLRequired: boolean;
}

const providerIDs = new Set<string>(LLM_PROVIDER_IDS);

export function isLLMProvider(value: string | undefined): value is LLMProvider {
  return Boolean(value && providerIDs.has(value));
}

export function normalizeProvider(value: string | undefined): AgentProvider {
  return isLLMProvider(value) ? value : "none";
}

export function normalizeProviderDescriptors(
  values: Schemas["dto.LLMProviderDescriptorDTO"][] | undefined,
): LLMProviderDescriptor[] {
  const seen = new Set<LLMProvider>();
  const normalized: LLMProviderDescriptor[] = [];
  for (const value of values ?? []) {
    if (!isLLMProvider(value.id) || seen.has(value.id)) continue;
    seen.add(value.id);
    normalized.push({
      id: value.id,
      apiKeyRequired: Boolean(value.api_key_required),
      baseURLRequired: Boolean(value.base_url_required),
    });
  }
  return normalized;
}

export function findProviderDescriptor(
  providers: readonly LLMProviderDescriptor[],
  provider: AgentProvider,
): LLMProviderDescriptor | undefined {
  if (provider === "none") return undefined;
  return providers.find((candidate) => candidate.id === provider);
}
