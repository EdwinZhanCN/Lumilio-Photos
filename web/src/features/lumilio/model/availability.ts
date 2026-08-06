import type { AgentServerAvailability } from "@/lib/capabilities/types";

export type { AgentServerAvailability } from "@/lib/capabilities/types";
export type AgentUIAvailability =
  | "checking"
  | "disabled"
  | "not_configured"
  | "ready"
  | "busy"
  | "degraded"
  | "unreachable";

export function resolveAgentAvailability(input: {
  server?: AgentServerAvailability;
  isLoading: boolean;
  isError: boolean;
  isGenerating: boolean;
  hasRuntimeError?: boolean;
}): AgentUIAvailability {
  if (input.isError) return "unreachable";
  if (!input.server) return input.isLoading ? "checking" : "unreachable";
  if (input.server !== "ready") return input.server;
  if (input.isGenerating) return "busy";
  return input.hasRuntimeError ? "degraded" : "ready";
}
