# Studio Tools

Built-in image processing tools for Studio. Each tool is a self-contained module with a UI panel and a runner function. The border tool renders entirely on an OffscreenCanvas in the worker (no wasm).

## Architecture

```
studio/tools/
├── types.ts                  # Shared tool contracts
├── border/                   # Border tool (5 modes)
│   ├── index.ts              # Public API
│   ├── types.ts              # BorderParams, normalizeParams, modes
│   ├── BorderPanel.tsx       # React UI panel
│   ├── borderRunner.ts       # Orchestration (routes modes to renderers)
│   ├── canvasUtils.ts        # Shared worker-safe OffscreenCanvas helpers
│   ├── basicBorders.ts       # COLORED / VIGNETTE / FROSTED renderers
│   ├── exifBorderRenderer.ts # FROSTED_INFO / INFO_STRIP renderers
│   ├── exifInfo.ts           # EXIF extraction/formatting (pure, worker-safe)
│   └── logoAssets.ts         # Brand match + SVG->ImageBitmap (MAIN THREAD only)
└── <next-tool>/              # Future tools follow the same shape
```

Worker boundary: `tool.worker.ts` -> `borderRunner.ts` may only import worker-safe
modules (`types`, `canvasUtils`, `basicBorders`, `exifBorderRenderer`, `exifInfo`).
`logoAssets.ts`, `BorderPanel.tsx`, and the `index.ts` barrel use the DOM and must
stay on the main thread.

Tools run off the main thread in `web/src/workers/tool.worker.ts`. The main thread communicates via `AppWorkerClient` (`web/src/workers/workerClient.ts`).

## Adding a New Tool

### 1. Create the tool directory

```txt
studio/tools/my-tool/
├── index.ts
├── types.ts
├── MyToolPanel.tsx
└── myToolRunner.ts
```

### 2. Define params and runner

`types.ts` — param type, defaults, normalizer:

```ts
export type MyToolParams = { strength: number; color: string };

export const DEFAULT_PARAMS: MyToolParams = { strength: 1.0, color: "#000" };

export function normalizeParams(raw: Record<string, unknown>): MyToolParams {
  return {
    strength: typeof raw.strength === "number" ? raw.strength : DEFAULT_PARAMS.strength,
    color: typeof raw.color === "string" ? raw.color : DEFAULT_PARAMS.color,
  };
}
```

`myToolRunner.ts` — export a single async function matching `ToolRunner`:

```ts
import type { ToolRunner } from "../types";

export const runMyTool: ToolRunner = async (ctx, params, helpers) => {
  const { inputFile, signal } = ctx;
  helpers?.reportProgress?.(1, 3);

  // ... process image bytes ...

  return { bytes: outputBytes, mimeType: "image/png", fileName: "output.png" };
};
```

### 3. Build the UI panel

`MyToolPanel.tsx` — a React component receiving `{ value, onChange, disabled }`:

```tsx
export const MyToolPanel: React.FC<{
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
}> = ({ value, onChange, disabled }) => {
  const p = normalizeParams(value);
  return (/* sliders, inputs, etc. */);
};
```

### 4. Export from `index.ts`

```ts
export { MyToolPanel } from "./MyToolPanel";
export { runMyTool } from "./myToolRunner";
export { normalizeParams, DEFAULT_PARAMS } from "./types";
```

### 5. Register in the tool worker

In `web/src/workers/tool.worker.ts`, import the runner and add it to `registerBuiltinTools()`:

```ts
import { runMyTool } from "@/features/studio/modules/tools/my-tool/myToolRunner";

function registerBuiltinTools(): void {
  if (toolRunners.size > 0) return;
  toolRunners.set("border", (ctx, params, helpers) => /* ... */);
  toolRunners.set("my-tool", (ctx, params, helpers) =>
    runMyTool(ctx, params, helpers),
  );
}
```

### 6. Wire into Studio

In `Studio.tsx`, add state and a handler; in `StudioSidebar.tsx`, add a nav entry; in `StudioToolsPanel.tsx`, add a render branch for the new panel.

## Worker Protocol

Messages from main thread → worker:

| Type        | Payload                               | Description                                                          |
| ----------- | ------------------------------------- | -------------------------------------------------------------------- |
| `LOAD_TOOL` | `{ toolId }`                          | Pre-registers a tool; replies `TOOL_LOADED`                          |
| `RUN_TOOL`  | `{ requestId, toolId, file, params }` | Executes a tool; replies `TOOL_PROGRESS` / `TOOL_COMPLETE` / `ERROR` |
| `ABORT`     | —                                     | Aborts the active run                                                |

Messages from worker → main thread:

| Type            | Payload                                    |
| --------------- | ------------------------------------------ |
| `TOOL_LOADED`   | `{ toolId }`                               |
| `TOOL_PROGRESS` | `{ requestId, processed, total }`          |
| `TOOL_COMPLETE` | `{ requestId, bytes, mimeType, fileName }` |
| `ERROR`         | `{ stage, requestId?, toolId?, error }`    |

## ToolRunner Contract

```ts
type ToolRunner = (
  ctx: { inputFile: File; signal: AbortSignal },
  params: Record<string, unknown>,
  helpers?: { reportProgress?: (processed: number, total: number) => void },
) => Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }>;
```

Runners must:

- Check `ctx.signal.aborted` at safe points and throw if true.
- Report progress via `helpers.reportProgress(current, total)`.
- Return raw image bytes with a valid MIME type.

## WorkerClient API

```ts
const workerClient = useWorker();

await workerClient.loadTool("border"); // optional, runTool calls it internally
const result = await workerClient.runTool("border", file, params); // { blob, fileName, mimeType }
workerClient.abortTool(); // cancel active run
```

`result.blob` is an in-memory `Blob` ready for `URL.createObjectURL()` or download.
