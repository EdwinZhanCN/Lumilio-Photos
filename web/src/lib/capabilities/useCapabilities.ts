import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";
import type { UseQueryResult } from "@tanstack/react-query";
import type { AgentServerAvailability } from "./types";

type CapabilitiesResponseDTO = components["schemas"]["dto.CapabilitiesResponseDTO"];

type MLTaskCapability = {
  enabled: boolean;
  available: boolean;
};

export type Capabilities = {
  ml: {
    discoveredNodeCount: number;
    activeNodeCount: number;
    tasks: {
      clipImageEmbed: MLTaskCapability;
      semanticTextEmbed: MLTaskCapability;
      bioClipClassify: MLTaskCapability;
      ocr: MLTaskCapability;
      faceDetectAndEmbed: MLTaskCapability;
    };
  };
  llm: {
    availability: AgentServerAvailability;
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

function normalizeCapabilities(data?: CapabilitiesResponseDTO): Capabilities | undefined {
  if (!data) {
    return undefined;
  }

  return {
    ml: {
      discoveredNodeCount: data.ml?.discovered_node_count ?? 0,
      activeNodeCount: data.ml?.active_node_count ?? 0,
      tasks: {
        clipImageEmbed: normalizeTaskCapability(data.ml?.tasks?.semantic_image_embed),
        semanticTextEmbed: normalizeTaskCapability(data.ml?.tasks?.semantic_text_embed),
        bioClipClassify: normalizeTaskCapability(data.ml?.tasks?.bioclip_classify),
        ocr: normalizeTaskCapability(data.ml?.tasks?.ocr),
        faceDetectAndEmbed: normalizeTaskCapability(data.ml?.tasks?.face_recognition),
      },
    },
    llm: {
      availability: (() => {
        const wire = (data.llm as typeof data.llm & { availability?: string } | undefined)
          ?.availability;
        if (wire === "disabled" || wire === "not_configured" || wire === "ready") return wire;
        if (!data.llm?.agent_enabled) return "disabled";
        return data.llm?.configured ? "ready" : "not_configured";
      })(),
      agentEnabled: Boolean(data.llm?.agent_enabled),
      configured: Boolean(data.llm?.configured),
      provider: data.llm?.provider ?? "",
      modelName: data.llm?.model_name ?? "",
    },
  };
}

export function useCapabilities(refetchInterval?: number | false): UseQueryResult<
  CapabilitiesResponseDTO,
  unknown
> & {
  capabilities?: Capabilities;
} {
  const query = $api.useQuery(
    "get",
    "/api/v1/capabilities",
    {},
    {
      refetchInterval,
    },
  ) as UseQueryResult<CapabilitiesResponseDTO, unknown>;

  return {
    ...query,
    capabilities: normalizeCapabilities(query.data),
  };
}

export { defaultTaskCapability };
