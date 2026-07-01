/// <reference lib="webworker" />

import { runBorderTransform } from "@/features/studio/tools/border/borderRunner";
import type { ToolRunner } from "@/features/studio/tools/types";

interface LoadToolMessage {
  type: "LOAD_TOOL";
  payload: {
    toolId: string;
  };
}

interface RunToolMessage {
  type: "RUN_TOOL";
  payload: {
    requestId: number;
    toolId: string;
    file: File;
    params: Record<string, unknown>;
  };
}

interface AbortMessage {
  type: "ABORT";
}

type ToolWorkerMessage = LoadToolMessage | RunToolMessage | AbortMessage;

const toolRunners: Map<string, ToolRunner> = new Map();
let activeAbortController: AbortController | null = null;
let activeRequestId: number | null = null;

function registerBuiltinTools(): void {
  if (toolRunners.size > 0) return;

  toolRunners.set("border", (ctx, params, helpers) =>
    runBorderTransform(ctx.inputFile, ctx.signal, params, helpers),
  );
}

self.onmessage = async (event: MessageEvent<ToolWorkerMessage>) => {
  const { type } = event.data;

  if (type === "ABORT") {
    if (activeAbortController) {
      activeAbortController.abort();
    }
    return;
  }

  if (type === "LOAD_TOOL") {
    const { toolId } = event.data.payload;
    registerBuiltinTools();

    if (toolRunners.has(toolId)) {
      self.postMessage({
        type: "TOOL_LOADED",
        payload: { toolId },
      });
    } else {
      self.postMessage({
        type: "ERROR",
        payload: {
          stage: "load_tool",
          toolId,
          error: `Unknown tool: ${toolId}`,
        },
      });
    }
    return;
  }

  if (type === "RUN_TOOL") {
    const { requestId, toolId, file, params } = event.data.payload;
    registerBuiltinTools();

    const runner = toolRunners.get(toolId);
    if (!runner) {
      self.postMessage({
        type: "ERROR",
        payload: {
          stage: "run_tool",
          requestId,
          toolId,
          error: `Unknown tool: ${toolId}`,
        },
      });
      return;
    }

    try {
      activeAbortController = new AbortController();
      activeRequestId = requestId;

      const result = await runner(
        {
          inputFile: file,
          signal: activeAbortController.signal,
        },
        params,
        {
          reportProgress: (processed, total) => {
            self.postMessage({
              type: "TOOL_PROGRESS",
              payload: { requestId, processed, total },
            });
          },
        },
      );

      if (activeAbortController.signal.aborted) {
        throw new Error("Operation aborted");
      }

      const bytes =
        result.bytes instanceof Uint8Array ? result.bytes : new Uint8Array(result.bytes);

      self.postMessage(
        {
          type: "TOOL_COMPLETE",
          payload: {
            requestId,
            fileName: result.fileName || "tool-output.bin",
            mimeType: result.mimeType || "application/octet-stream",
            bytes,
          },
        },
        [bytes.buffer],
      );
    } catch (error) {
      self.postMessage({
        type: "ERROR",
        payload: {
          stage: "run_tool",
          requestId,
          toolId,
          error: error instanceof Error ? error.message : "Tool execution failed",
        },
      });
    } finally {
      if (activeRequestId === requestId) {
        activeAbortController = null;
        activeRequestId = null;
      }
    }
  }
};

export {};
