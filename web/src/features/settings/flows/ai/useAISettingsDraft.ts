import type { UpdateSystemSettings } from "../../api/useSystemSettings";
import { useSystemSettings, useUpdateSystemSettings } from "../../api/useSystemSettings";
import { useDraftSettings, type DraftSettings } from "../../hooks/useDraftSettings";

type AgentProvider = "" | "ark" | "openai" | "deepseek" | "ollama";

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
    case "":
    case "openai":
    case "deepseek":
    case "ollama":
    case "ark":
      return value;
    default:
      return "";
  }
}

function buildPayload(draft: AISettingsDraft): UpdateSystemSettings {
  const payload: UpdateSystemSettings = {
    llm: {
      agent_enabled: draft.llm.agentEnabled,
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

  if (draft.llm.provider) {
    payload.llm = {
      ...payload.llm,
      provider: draft.llm.provider,
    };
  }

  if (draft.llm.clearStoredKey) {
    payload.llm = {
      ...payload.llm,
      api_key: "",
    };
  } else if (draft.llm.apiKey.trim()) {
    payload.llm = {
      ...payload.llm,
      api_key: draft.llm.apiKey.trim(),
    };
  }

  return payload;
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
} {
  const query = useSystemSettings();
  const mutation = useUpdateSystemSettings();
  const server = query.data ? toServerDraft(query.data) : undefined;

  const draftSettings = useDraftSettings<AISettingsDraft>({
    server,
    isLoading: query.isLoading,
    isSaving: mutation.isPending,
    saveError: mutation.error,
    onSave: async (draft) => {
      await mutation.mutateAsync({ body: buildPayload(draft) });
    },
  });

  return {
    ...draftSettings,
    apiKeyConfigured: Boolean(query.data?.llm?.api_key_configured),
    query,
  };
}
