// src/services/agentService.ts

import client from "@/lib/http-commons/client";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];

export type AgentChatRequest = Schemas["handler.AgentChatRequest"];
export type AgentResumeRequest = Schemas["handler.AgentResumeRequest"];
export type ToolInfoResponse = Schemas["handler.ToolInfoResponse"];
export type ToolSchemaResponse = Schemas["handler.ToolSchemaResponse"];

/**
 * Agent event from SSE stream
 */
export interface AgentEvent {
  agent_name: string;
  run_path?: string[];
  output?: string;
  reasoning?: string;
  action?: {
    name: string;
    input: unknown;
    interrupted?: unknown;
  };
  error?: string;
}

/**
 * SSE event type
 */
export type AgentEventType =
  | "session_info"
  | "message"
  | "action"
  | "ui_event"
  | "done"
  | "error";

/**
 * SSE stream event
 */
export interface AgentStreamEvent {
  type: AgentEventType;
  data: unknown;
}

// Base URL for SSE streaming
const baseURL = import.meta.env.VITE_API_URL || "http://localhost:8080";

// ============================================================================
// Agent Service
// ============================================================================

export const agentService = {
  /**
   * Get the base URL for API requests
   */
  getBaseUrl(): string {
    return baseURL;
  },

  /**
   * Private method to handle SSE streaming for both chat and resume
   * Note: SSE streaming uses raw fetch, not openapi-fetch
   */
  async *_streamer(
    url: string,
    body: object,
    signal?: AbortSignal,
  ): AsyncGenerator<AgentStreamEvent> {
    try {
      const response = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
        signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error("No response body");
      }

      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();

        if (done) {
          break;
        }

        buffer += decoder.decode(value, { stream: true });

        // Process SSE messages
        const messages = buffer.split("\n\n");
        buffer = messages.pop() || ""; // Keep incomplete message in buffer

        for (const message of messages) {
          if (!message.trim()) continue;

          const event = this.parseSSEEvent(message);
          if (event) {
            yield event;
          }
        }
      }
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") {
        return; // Request was aborted, this is expected
      }
      throw error;
    }
  },

  /**
   * Sends a chat request to the agent and returns a stream of events
   */
  async *streamAgentChat(
    request: AgentChatRequest,
    signal?: AbortSignal,
  ): AsyncGenerator<AgentStreamEvent> {
    const url = `${this.getBaseUrl()}/api/v1/agent/chat`;
    yield* this._streamer(url, request, signal);
  },

  /**
   * Sends a resume request to the agent and returns a stream of events
   */
  async *streamAgentResume(
    request: AgentResumeRequest,
    signal?: AbortSignal,
  ): AsyncGenerator<AgentStreamEvent> {
    const url = `${this.getBaseUrl()}/api/v1/agent/chat/resume`;
    yield* this._streamer(url, request, signal);
  },

  /**
   * Parses an SSE event string into an AgentStreamEvent
   */
  parseSSEEvent(message: string): AgentStreamEvent | null {
    const eventLine = message
      .split("\n")
      .find((line) => line.startsWith("event:"));
    const dataLine = message
      .split("\n")
      .find((line) => line.startsWith("data:"));

    if (!eventLine || !dataLine) {
      return null;
    }

    const eventType = eventLine
      .substring("event: ".length)
      .trim() as AgentEventType;
    const dataStr = dataLine.substring("data: ".length).trim();

    try {
      const data = JSON.parse(dataStr);
      return { type: eventType, data };
    } catch (error) {
      console.error("Failed to parse SSE data:", error, "Data:", dataStr);
      return null;
    }
  },

  /**
   * Gets the list of available tools
   */
  async getAvailableTools() {
    const { data } = await client.GET("/api/v1/agent/tools", {});
    return data?.data as ToolInfoResponse[] | undefined;
  },

  /**
   * Gets tool schemas
   */
  async getToolSchemas() {
    const { data } = await client.GET("/api/v1/agent/schemas", {});
    return data?.data as ToolSchemaResponse | undefined;
  },

  /**
   * Gets tools formatted for slash commands
   */
  async getSlashCommands(): Promise<
    Array<{
      id: string;
      label: string;
      type: "command";
      meta: string;
      description?: string;
    }>
  > {
    try {
      const tools = await this.getAvailableTools();

      if (!tools || !Array.isArray(tools)) {
        console.warn("Invalid tools response:", tools);
        return [];
      }

      return tools
        .filter((tool) => tool.name)
        .map((tool) => ({
          id: tool.name!,
          label: tool.name!,
          type: "command" as const,
          meta: tool.desc || "No description available",
          description: tool.desc,
        }));
    } catch (error) {
      console.error("Error fetching slash commands:", error);
      return [];
    }
  },
};

// ============================================================================
// React Query Hooks
// ============================================================================

/**
 * Hook for available tools
 */
export const useAgentTools = () =>
  $api.useQuery("get", "/api/v1/agent/tools", {});

/**
 * Hook for tool schemas
 */
export const useToolSchemas = () =>
  $api.useQuery("get", "/api/v1/agent/schemas", {});

export default agentService;
