// src/services/agentService.ts

import api from "@/lib/http-commons/api";
import type { components } from "@/lib/http-commons/schema";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];

/**
 * Agent chat request
 */
export type AgentChatRequest = Schemas["handler.AgentChatRequest"];

/**
 * Tool info response
 */
export type ToolInfoResponse = Schemas["handler.ToolInfoResponse"];

/**
 * API result wrapper
 */
export type ApiResult<T> = {
  code: number;
  message: string;
  data: T;
};

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
  };
  error?: string;
}

/**
 * SSE event type
 */
export type AgentEventType = "message" | "done" | "error";

/**
 * SSE stream event
 */
export interface AgentStreamEvent {
  type: AgentEventType;
  data: AgentEvent | { error: string };
}

// ============================================================================
// Agent Service
// ============================================================================

/**
 * Agent Service for handling AI agent interactions
 */
export const agentService = {
  /**
   * Get the base URL for API requests
   */
  getBaseUrl(): string {
    return (
      import.meta.env.VITE_API_URL ||
      import.meta.env.API_URL ||
      "http://localhost:8080"
    );
  },

  /**
   * Sends a chat request to the agent and returns a stream of events
   *
   * @param request - Chat request with query and optional tool names
   * @param signal - Optional abort signal to cancel the request
   * @returns Async generator that yields stream events
   */
  async *streamAgentChat(
    request: AgentChatRequest,
    signal?: AbortSignal,
  ): AsyncGenerator<AgentStreamEvent> {
    const url = `${this.getBaseUrl()}/api/v1/agent/chat`;

    try {
      const response = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(request),
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
        const lines = buffer.split("\n\n");
        buffer = lines.pop() || ""; // Keep incomplete message in buffer

        for (const line of lines) {
          if (!line.trim()) continue;

          // Debug logging for troubleshooting
          // console.log(
          //   "[DEBUG] Raw SSE line:",
          //   line.substring(0, 200) + (line.length > 200 ? "..." : ""),
          // );

          const event = this.parseSSEEvent(line);
          if (event) {
            // Debug logging for troubleshooting
            // console.log("[DEBUG] Parsed event:", {
            //   type: event.type,
            //   data: event.data,
            //   dataType: typeof event.data,
            //   outputLength:
            //     event.data &&
            //     typeof event.data === "object" &&
            //     "output" in event.data
            //       ? typeof event.data.output === "string"
            //         ? event.data.output.length
            //         : "not string"
            //       : "no output",
            // });
            yield event;
          }
        }
      }
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") {
        // Request was aborted, this is expected
        return;
      }

      throw error;
    }
  },

  /**
   * Parses an SSE event string into an AgentStreamEvent
   *
   * @param line - Raw SSE event string
   * @returns Parsed event or null if invalid
   */
  parseSSEEvent(line: string): AgentStreamEvent | null {
    const eventMatch = line.match(/^event:\s*(.+)$/m);
    const dataMatch = line.match(/^data:\s*(.+)$/m);

    if (!eventMatch || !dataMatch) {
      // console.log("[DEBUG] Failed to parse SSE line - missing event or data");
      return null;
    }

    const eventType = eventMatch[1].trim() as AgentEventType;
    const dataStr = dataMatch[1].trim();

    // console.log(
    //   "[DEBUG] Parsing SSE - event:",
    //   eventType,
    //   "data length:",
    //   dataStr.length,
    // );

    try {
      const data = JSON.parse(dataStr);

      // Handle done event (data is {})
      if (eventType === "done") {
        // console.log("[DEBUG] Done event received");
        return { type: "done", data: {} as AgentEvent };
      }

      // console.log("[DEBUG] Parsed data keys:", Object.keys(data));
      // if (data.output && typeof data.output === "string") {
      //   console.log(
      //     "[DEBUG] Output text preview:",
      //     data.output.substring(0, 100) +
      //       (data.output.length > 100 ? "..." : ""),
      //   );
      // }

      return { type: eventType, data };
    } catch (error) {
      console.error(
        "Failed to parse SSE data:",
        error,
        "Data string:",
        dataStr.substring(0, 200),
      );
      return null;
    }
  },

  /**
   * Gets the list of available tools
   *
   * @returns Promise resolving to array of tool info
   */
  async getAvailableTools(): Promise<ToolInfoResponse[]> {
    const response =
      await api.get<ApiResult<ToolInfoResponse[]>>("/agent/tools");
    return response.data.data;
  },
};

export default agentService;
