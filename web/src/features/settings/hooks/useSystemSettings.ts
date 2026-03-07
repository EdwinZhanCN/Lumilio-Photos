import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

type SystemSettingsResponseDTO = Schemas["dto.SystemSettingsDTO"];
export type UpdateSystemSettingsPayload =
  Schemas["dto.UpdateSystemSettingsDTO"];

export type SystemSettings = {
  llm: {
    agentEnabled: boolean;
    provider: "" | "ark" | "openai" | "deepseek" | "ollama";
    modelName: string;
    baseURL: string;
    apiKeyConfigured: boolean;
  };
  ml: {
    autoMode: "enable" | "disable";
    clipEnabled: boolean;
    ocrEnabled: boolean;
    captionEnabled: boolean;
    faceEnabled: boolean;
  };
  updatedAt: string;
  updatedBy?: number;
};

function normalizeProvider(
  value: string | undefined,
): SystemSettings["llm"]["provider"] {
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

function normalizeSystemSettings(
  data?: SystemSettingsResponseDTO,
): SystemSettings | undefined {
  if (!data) {
    return undefined;
  }

  return {
    llm: {
      agentEnabled: Boolean(data.llm?.agent_enabled),
      provider: normalizeProvider(data.llm?.provider),
      modelName: data.llm?.model_name ?? "",
      baseURL: data.llm?.base_url ?? "",
      apiKeyConfigured: Boolean(data.llm?.api_key_configured),
    },
    ml: {
      autoMode: data.ml?.auto_mode === "enable" ? "enable" : "disable",
      clipEnabled: Boolean(data.ml?.clip_enabled),
      ocrEnabled: Boolean(data.ml?.ocr_enabled),
      captionEnabled: Boolean(data.ml?.caption_enabled),
      faceEnabled: Boolean(data.ml?.face_enabled),
    },
    updatedAt: data.updated_at ?? "",
    updatedBy: data.updated_by ?? undefined,
  };
}

export function useSystemSettings(): UseQueryResult<
  ApiResult<SystemSettingsResponseDTO>,
  unknown
> & { settings?: SystemSettings } {
  const query = $api.useQuery(
    "get",
    "/api/v1/settings/system",
    {},
    {
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<SystemSettingsResponseDTO>, unknown>;

  return {
    ...query,
    settings: normalizeSystemSettings(query.data?.data),
  };
}

export function useUpdateSystemSettings() {
  const queryClient = useQueryClient();

  return $api.useMutation("patch", "/api/v1/settings/system", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/settings/system"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/capabilities"],
        }),
      ]);
    },
  });
}

export function useValidateLLMSettings() {
  return $api.useMutation("post", "/api/v1/settings/system/validate-llm");
}
