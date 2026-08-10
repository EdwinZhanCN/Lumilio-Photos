import type {
  UpdateSystemSettings,
  ValidateLLMDraft,
} from "../../api/useSystemSettings";
import {
  useSystemSettings,
  useUpdateSystemSettings,
  useValidateLLMSettings,
} from "../../api/useSystemSettings";
import { useDraftSettings, type DraftSettings } from "../../hooks/useDraftSettings";

type AgentProvider = "none" | "ark" | "openai" | "deepseek" | "ollama";

export interface AISettingsDraft {
  llm: {
    agentEnabled: boolean;
    provider: AgentProvider;
    modelName: string;
    baseURL: string;
    apiKey: string;
    clearStoredKey: boolean;
  };
  ml: {
    semanticEnabled: boolean;
    videoSemanticEnabled: boolean;
    bioclipEnabled: boolean;
    ocrEnabled: boolean;
    faceEnabled: boolean;
  };
}

function normalizeProvider(value: string | undefined): AgentProvider {
  switch (value) {
    case "openai":
    case "deepseek":
    case "ollama":
    case "ark":
      return value;
    default:
      return "none";
  }
}

function buildPayload(draft: AISettingsDraft): UpdateSystemSettings {
  const payload: UpdateSystemSettings = {
    llm: {
      agent_enabled: draft.llm.agentEnabled,
      // `none` is an explicit update sentinel and is normalized to an empty
      // stored provider by the service.
      provider: draft.llm.provider,
      model_name: draft.llm.modelName.trim(),
      base_url: draft.llm.baseURL.trim(),
    },
    ml: {
      semantic_enabled: draft.ml.semanticEnabled,
      video_semantic_enabled: draft.ml.videoSemanticEnabled,
      bioclip_enabled: draft.ml.bioclipEnabled,
      ocr_enabled: draft.ml.ocrEnabled,
      face_enabled: draft.ml.faceEnabled,
    },
  };

  if (draft.llm.clearStoredKey) {
    payload.llm = { ...payload.llm, api_key: "" };
  } else if (draft.llm.apiKey.trim()) {
    payload.llm = { ...payload.llm, api_key: draft.llm.apiKey.trim() };
  }
  return payload;
}

export function buildValidationPayload(
  draft: AISettingsDraft,
  apiKeyConfigured: boolean,
  storedProvider: AgentProvider,
): ValidateLLMDraft {
  if (draft.llm.provider === "none") {
    throw new Error("Select an LLM provider before validation");
  }
  if (!draft.llm.modelName.trim()) {
    throw new Error("Enter a model name before validation");
  }
  if (
    (draft.llm.provider === "ollama" || draft.llm.provider === "deepseek") &&
    !draft.llm.baseURL.trim()
  ) {
    throw new Error("Enter the provider base URL before validation");
  }
  const apiKey = draft.llm.clearStoredKey ? "" : draft.llm.apiKey.trim();
  const canUseStoredKey =
    !apiKey &&
    !draft.llm.clearStoredKey &&
    apiKeyConfigured &&
    draft.llm.provider === storedProvider;
  if (draft.llm.provider !== "ollama" && !apiKey && !canUseStoredKey) {
    throw new Error("Enter an API key for the selected provider before validation");
  }
  return {
    provider: draft.llm.provider,
    model_name: draft.llm.modelName.trim(),
    base_url: draft.llm.baseURL.trim(),
    ...(apiKey ? { api_key: apiKey } : {}),
    use_stored_api_key: canUseStoredKey,
  };
}

function toServerDraft(
  data: NonNullable<ReturnType<typeof useSystemSettings>["data"]>,
): AISettingsDraft | undefined {
  if (!data) return undefined;
  return {
    llm: {
      agentEnabled: Boolean(data.llm?.agent_enabled),
      provider: normalizeProvider(data.llm?.provider),
      modelName: data.llm?.model_name ?? "",
      baseURL: data.llm?.base_url ?? "",
      apiKey: "",
      clearStoredKey: false,
    },
    ml: {
      semanticEnabled: Boolean(data.ml?.semantic_enabled),
      videoSemanticEnabled: Boolean(data.ml?.video_semantic_enabled),
      bioclipEnabled: Boolean(data.ml?.bioclip_enabled),
      ocrEnabled: Boolean(data.ml?.ocr_enabled),
      faceEnabled: Boolean(data.ml?.face_enabled),
    },
  };
}

export function useAISettingsDraft(): DraftSettings<AISettingsDraft> & {
  apiKeyConfigured: boolean;
  query: ReturnType<typeof useSystemSettings>;
  isValidating: boolean;
  validateDraft: () => Promise<void>;
} {
  const query = useSystemSettings();
  const saveMutation = useUpdateSystemSettings();
  const validateMutation = useValidateLLMSettings();
  const server = query.data ? toServerDraft(query.data) : undefined;
  const apiKeyConfigured = Boolean(query.data?.llm?.api_key_configured);
  const storedProvider = normalizeProvider(query.data?.llm?.provider);

  const draftSettings = useDraftSettings<AISettingsDraft>({
    server,
    isLoading: query.isLoading,
    isSaving: saveMutation.isPending || validateMutation.isPending,
    saveError: saveMutation.error ?? validateMutation.error,
    onSave: async (draft) => {
      if (draft.llm.agentEnabled) {
        await validateMutation.mutateAsync(
          buildValidationPayload(draft, apiKeyConfigured, storedProvider),
        );
      }
      await saveMutation.mutateAsync({ body: buildPayload(draft) });
    },
  });

  return {
    ...draftSettings,
    apiKeyConfigured,
    query,
    isValidating: validateMutation.isPending,
    validateDraft: async () => {
      if (!draftSettings.draft) return;
      await validateMutation.mutateAsync(
        buildValidationPayload(draftSettings.draft, apiKeyConfigured, storedProvider),
      );
    },
  };
}
