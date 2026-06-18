import {
  useSystemSettings,
  useUpdateSystemSettings,
} from "./useSystemSettings";
import { useDraftSettings, type DraftSettings } from "./useDraftSettings";

export interface LLMDraft {
  agent_enabled: boolean;
  provider: string;
  model_name: string;
  base_url: string;
  api_key?: string;
}

export function useLLMSettingsDraft(): DraftSettings<LLMDraft> {
  const query = useSystemSettings();
  const mutation = useUpdateSystemSettings();

  const llm = query.data?.llm;
  const server: LLMDraft | undefined = llm
    ? {
        agent_enabled: llm.agent_enabled ?? false,
        provider: llm.provider ?? "ark",
        model_name: llm.model_name ?? "",
        base_url: llm.base_url ?? "",
      }
    : undefined;

  return useDraftSettings<LLMDraft>({
    server,
    isLoading: query.isLoading,
    isSaving: mutation.isPending,
    saveError: mutation.error,
    onSave: (draft) =>
      mutation.mutateAsync({
        body: {
          llm: {
            agent_enabled: draft.agent_enabled,
            provider: draft.provider as
              | "ark"
              | "openai"
              | "deepseek"
              | "ollama",
            model_name: draft.model_name,
            base_url: draft.base_url,
            ...(draft.api_key !== undefined ? { api_key: draft.api_key } : {}),
          },
        },
      }),
  });
}
