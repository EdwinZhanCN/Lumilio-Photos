import { $api } from "@/lib/http-commons/queryClient";
import type { components, paths } from "@/lib/http-commons/schema";
import type { UseQueryResult } from "@tanstack/react-query";

type CapabilitiesResponseDTO =
  components["schemas"]["dto.CapabilitiesResponseDTO"];

type CapabilitiesApiResult = Omit<
  paths["/api/v1/capabilities"]["get"]["responses"][200]["content"]["application/json"],
  "data"
> & {
  data?: CapabilitiesResponseDTO;
};

type MLTaskCapability = {
  enabled: boolean;
  available: boolean;
};

export type Capabilities = {
  ml: {
    autoMode: "enable" | "disable";
    discoveredNodeCount: number;
    activeNodeCount: number;
    tasks: {
      clipImageEmbed: MLTaskCapability;
      ocr: MLTaskCapability;
      vlmGenerate: MLTaskCapability;
      faceDetectAndEmbed: MLTaskCapability;
    };
  };
  llm: {
    agentEnabled: boolean;
    configured: boolean;
    provider: string;
    modelName: string;
  };
};

const defaultTaskCapability = (): MLTaskCapability => ({
  enabled: false,
  available: false,
});

function normalizeTaskCapability(
  task?: components["schemas"]["dto.MLTaskCapabilityDTO"],
): MLTaskCapability {
  return {
    enabled: Boolean(task?.enabled),
    available: Boolean(task?.available),
  };
}

function normalizeCapabilities(
  data?: CapabilitiesResponseDTO,
): Capabilities | undefined {
  if (!data) {
    return undefined;
  }

  return {
    ml: {
      autoMode: data.ml?.auto_mode === "enable" ? "enable" : "disable",
      discoveredNodeCount: data.ml?.discovered_node_count ?? 0,
      activeNodeCount: data.ml?.active_node_count ?? 0,
      tasks: {
        clipImageEmbed: normalizeTaskCapability(
          data.ml?.tasks?.clip_image_embed,
        ),
        ocr: normalizeTaskCapability(data.ml?.tasks?.ocr),
        vlmGenerate: normalizeTaskCapability(data.ml?.tasks?.vlm_generate),
        faceDetectAndEmbed: normalizeTaskCapability(
          data.ml?.tasks?.face_detect_and_embed,
        ),
      },
    },
    llm: {
      agentEnabled: Boolean(data.llm?.agent_enabled),
      configured: Boolean(data.llm?.configured),
      provider: data.llm?.provider ?? "",
      modelName: data.llm?.model_name ?? "",
    },
  };
}

export function useCapabilities(refetchInterval?: number | false): UseQueryResult<
  CapabilitiesApiResult,
  unknown
> & { capabilities?: Capabilities } {
  const query = $api.useQuery(
    "get",
    "/api/v1/capabilities",
    {},
    {
      refetchInterval,
    },
  ) as UseQueryResult<CapabilitiesApiResult, unknown>;

  return {
    ...query,
    capabilities: normalizeCapabilities(query.data?.data),
  };
}

export { defaultTaskCapability };
