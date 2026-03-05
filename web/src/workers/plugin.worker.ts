/// <reference lib="webworker" />

import type {
  PluginRunResult,
  RuntimeManifestV1,
  StudioPluginImageMimeType,
  StudioPluginRunnerModule,
} from "@/features/studio/plugins/types";
import {
  DEFAULT_STUDIO_PLUGIN_INPUT_MIME_TYPES,
  DEFAULT_STUDIO_PLUGIN_OUTPUT_MIME_TYPES,
  STUDIO_PLUGIN_IMAGE_MIME_TYPES,
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
const STUDIO_PLUGIN_IMAGE_MIME_TYPE_SET = new Set<string>(
  STUDIO_PLUGIN_IMAGE_MIME_TYPES,
);

function pluginKey(pluginId: string, version: string): string {
  return `${pluginId}@${version}`;
}

function isRunnerModule(value: unknown): value is StudioPluginRunnerModule {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.run === "function";
}

function normalizeMimeType(value: string): string {
  return value.trim().toLowerCase().split(";")[0] ?? "";
}

function getAllowedInputMimeTypes(
  manifest: RuntimeManifestV1,
): Set<StudioPluginImageMimeType> {
  const declared = manifest.io?.input?.mimeTypes;
  if (declared && declared.length > 0) {
    return new Set(declared);
  }
  return new Set(DEFAULT_STUDIO_PLUGIN_INPUT_MIME_TYPES);
}

function getAllowedOutputMimeTypes(
  manifest: RuntimeManifestV1,
): Set<StudioPluginImageMimeType> {
  const declared = manifest.io?.output?.mimeTypes;
  if (declared && declared.length > 0) {
    return new Set(declared);
  }
  return new Set(DEFAULT_STUDIO_PLUGIN_OUTPUT_MIME_TYPES);
}

function getPreferredOutputMimeType(
  manifest: RuntimeManifestV1,
): StudioPluginImageMimeType {
  return (
    manifest.io?.output?.preferredMimeType ||
    DEFAULT_STUDIO_PLUGIN_OUTPUT_MIME_TYPES[0]
  );
}

function assertInputMimeTypeSupported(
  file: File,
  manifest: RuntimeManifestV1,
): void {
  const mimeType = normalizeMimeType(file.type);
  const allowedInputMimeTypes = getAllowedInputMimeTypes(manifest);
  if (allowedInputMimeTypes.has(mimeType as StudioPluginImageMimeType)) {
    return;
  }

  throw new Error(
    `Unsupported input mimeType '${file.type || "<empty>"}'. Allowed: ${Array.from(allowedInputMimeTypes).join(", ")}`,
  );
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

function normalizeResult(
  result: PluginRunResult,
  manifest: RuntimeManifestV1,
): PluginRunResult {
  if (!(result.bytes instanceof Uint8Array)) {
    throw new Error("Runner result.bytes must be Uint8Array");
  }

  const fallbackMimeType = getPreferredOutputMimeType(manifest);
  const mimeType = normalizeMimeType(result.mimeType || fallbackMimeType);
  if (!STUDIO_PLUGIN_IMAGE_MIME_TYPE_SET.has(mimeType)) {
    throw new Error(
      `Runner result.mimeType '${result.mimeType}' is unsupported. Allowed platform output types: ${STUDIO_PLUGIN_IMAGE_MIME_TYPES.join(", ")}`,
    );
  }

  const allowedOutputMimeTypes = getAllowedOutputMimeTypes(manifest);
  if (!allowedOutputMimeTypes.has(mimeType as StudioPluginImageMimeType)) {
    throw new Error(
      `Runner result.mimeType '${mimeType}' is not allowed by manifest io.output.mimeTypes (${Array.from(allowedOutputMimeTypes).join(", ")})`,
    );
  }

  const fileName = result.fileName || "plugin-output.bin";

  return {
    bytes: result.bytes,
    mimeType: mimeType as StudioPluginImageMimeType,
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
          error:
            error instanceof Error ? error.message : "Failed to load runner",
        },
      });
    }

    return;
  }

  if (type === "RUN_PLUGIN") {
    const { requestId, pluginId, version, manifest, file, params } =
      event.data.payload;

    try {
      assertInputMimeTypeSupported(file, manifest);
      const runner = await loadRunner(
        pluginId,
        version,
        manifest.entries.runner,
      );

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

      const normalized = normalizeResult(result, manifest);
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
          error:
            error instanceof Error ? error.message : "Plugin execution failed",
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
