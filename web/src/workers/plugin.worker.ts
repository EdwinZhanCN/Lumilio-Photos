/// <reference lib="webworker" />

import type {
  PluginRunResult,
  RuntimeManifestV1,
  StudioPluginRunnerModule,
} from "@/features/studio/plugins/types";

interface LoadRunnerMessage {
  type: "LOAD_RUNNER";
  payload: {
    pluginId: string;
    version: string;
    runnerUrl: string;
  };
}

interface RunPluginMessage {
  type: "RUN_PLUGIN";
  payload: {
    requestId: number;
    pluginId: string;
    version: string;
    manifest: RuntimeManifestV1;
    file: File;
    params: Record<string, unknown>;
  };
}

interface AbortMessage {
  type: "ABORT";
  payload?: {
    requestId?: number;
  };
}

type PluginWorkerMessage = LoadRunnerMessage | RunPluginMessage | AbortMessage;

type LoadedRunnerMap = Map<string, StudioPluginRunnerModule>;

const loadedRunnerMap: LoadedRunnerMap = new Map();
let activeAbortController: AbortController | null = null;
let activeRequestId: number | null = null;

function pluginKey(pluginId: string, version: string): string {
  return `${pluginId}@${version}`;
}

function isRunnerModule(value: unknown): value is StudioPluginRunnerModule {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.run === "function";
}

async function loadRunner(
  pluginId: string,
  version: string,
  runnerUrl: string,
): Promise<StudioPluginRunnerModule> {
  const key = pluginKey(pluginId, version);
  const cached = loadedRunnerMap.get(key);
  if (cached) {
    return cached;
  }

  const mod = await import(/* @vite-ignore */ runnerUrl);
  const candidate =
    (isRunnerModule(mod?.default) ? mod.default : null) ||
    (isRunnerModule(mod?.runner) ? mod.runner : null) ||
    (isRunnerModule(mod) ? mod : null);

  if (!candidate) {
    throw new Error(`Runner module is invalid for ${key}`);
  }

  loadedRunnerMap.set(key, candidate);
  return candidate;
}

function normalizeResult(result: PluginRunResult): PluginRunResult {
  if (!(result.bytes instanceof Uint8Array)) {
    throw new Error("Runner result.bytes must be Uint8Array");
  }

  const mimeType = result.mimeType || "application/octet-stream";
  const fileName = result.fileName || "plugin-output.bin";

  return {
    bytes: result.bytes,
    mimeType,
    fileName,
  };
}

self.onmessage = async (event: MessageEvent<PluginWorkerMessage>) => {
  const { type } = event.data;

  if (type === "ABORT") {
    if (activeAbortController) {
      activeAbortController.abort();
    }
    return;
  }

  if (type === "LOAD_RUNNER") {
    const { pluginId, version, runnerUrl } = event.data.payload;
    try {
      await loadRunner(pluginId, version, runnerUrl);
      self.postMessage({
        type: "RUNNER_LOADED",
        payload: {
          pluginId,
          version,
        },
      });
    } catch (error) {
      self.postMessage({
        type: "ERROR",
        payload: {
          stage: "load_runner",
          pluginId,
          version,
          error: error instanceof Error ? error.message : "Failed to load runner",
        },
      });
    }

    return;
  }

  if (type === "RUN_PLUGIN") {
    const { requestId, pluginId, version, manifest, file, params } = event.data.payload;

    try {
      const runner = await loadRunner(pluginId, version, manifest.entries.runner);

      activeAbortController = new AbortController();
      activeRequestId = requestId;

      const result = await runner.run(
        {
          inputFile: file,
          signal: activeAbortController.signal,
          manifest,
        },
        params,
        {
          reportProgress: (processed, total) => {
            self.postMessage({
              type: "PLUGIN_PROGRESS",
              payload: {
                requestId,
                processed,
                total,
              },
            });
          },
        },
      );

      if (activeAbortController.signal.aborted) {
        throw new Error("Operation aborted");
      }

      const normalized = normalizeResult(result);
      const bytes = normalized.bytes;

      self.postMessage(
        {
          type: "PLUGIN_COMPLETE",
          payload: {
            requestId,
            fileName: normalized.fileName,
            mimeType: normalized.mimeType,
            bytes,
          },
        },
        [bytes.buffer],
      );
    } catch (error) {
      self.postMessage({
        type: "ERROR",
        payload: {
          stage: "run_plugin",
          requestId,
          pluginId,
          version,
          error: error instanceof Error ? error.message : "Plugin execution failed",
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
