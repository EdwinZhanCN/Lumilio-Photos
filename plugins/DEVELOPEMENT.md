# Lumilio Studio Plugin Development Guide

This document explains how to write and bundle a Studio plugin.

Out of scope:
- Key management
- Cloud resources and infrastructure
- Release/publish workflow

## 1) Plugin Runtime Model

A Studio plugin has two runtime entries:

- `UI Entry` (runs on main thread)
- `Runner Entry` (runs inside `plugin.worker`)

The host loads both entries from ESM files and validates their exported shape.

## 2) Required Exports

### UI Entry contract

Your UI module must export:

- `meta`
- `defaultParams`
- `Panel`
- optional `normalizeParams`

Minimal shape:

```ts
export const meta = {
  id: "com.example.hello",
  version: "0.1.0",
  displayName: "Hello Lumilio",
  mount: { panel: "plugins", order: 10 },
};

export const defaultParams: Record<string, unknown> = {};

export const Panel: React.FC<{
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
}> = () => null;

export const normalizeParams = (raw: Record<string, unknown>) => raw;
```

### Runner Entry contract

Your runner module must export `run(ctx, params, helpers?)` and return:

- `bytes: Uint8Array`
- `mimeType: string`
- `fileName: string`

Minimal shape:

```ts
export async function run(
  ctx: { inputFile: File; signal: AbortSignal },
  params: Record<string, unknown>,
  helpers?: { reportProgress?: (processed: number, total: number) => void },
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  // your processing logic
  return {
    bytes: new Uint8Array(),
    mimeType: "application/octet-stream",
    fileName: "output.bin",
  };
}
```

## 3) Recommended Folder Layout

```txt
plugins/your-plugin/
  package.json
  tsconfig.json
  src/
    ui.tsx
    runner.ts
    types.ts
  dist/
    ui.mjs
    runner.mjs
```

If your runner uses wasm, keep wasm sidecar files in `dist/` as well.

## 4) Bundle Rules

Use ESM output for both entries.

- UI bundle: externalize `react` and `react/jsx-runtime`
- Runner bundle: browser target, ESM format

Example:

```bash
node_modules/.bin/esbuild plugins/your-plugin/src/ui.tsx \
  --bundle \
  --format=esm \
  --platform=browser \
  --target=es2022 \
  --external:react \
  --external:react/jsx-runtime \
  --outfile=plugins/your-plugin/dist/ui.mjs

node_modules/.bin/esbuild plugins/your-plugin/src/runner.ts \
  --bundle \
  --format=esm \
  --platform=browser \
  --target=es2022 \
  --outfile=plugins/your-plugin/dist/runner.mjs
```

## 5) Hello Lumilio Plugin (Minimal Example)

This example keeps image bytes unchanged and only changes output file naming.

### `src/ui.tsx`

```tsx
import React from "react";

export const meta = {
  id: "com.lumilio.hello",
  version: "0.1.0",
  displayName: "Hello Lumilio",
  mount: {
    panel: "plugins" as const,
    order: 1,
  },
};

export const defaultParams: Record<string, unknown> = {
  suffix: "hello",
  uppercase: false,
};

export const normalizeParams = (raw: Record<string, unknown>) => {
  const suffix = typeof raw.suffix === "string" && raw.suffix.trim()
    ? raw.suffix.trim()
    : "hello";
  const uppercase = raw.uppercase === true;
  return { suffix, uppercase };
};

export const Panel: React.FC<{
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
}> = ({ value, onChange, disabled }) => {
  const v = normalizeParams(value);
  return (
    <div className="space-y-3">
      <label className="label">File suffix</label>
      <input
        className="input input-bordered w-full"
        value={v.suffix}
        disabled={disabled}
        onChange={(e) => onChange({ ...v, suffix: e.target.value })}
      />
      <label className="label cursor-pointer justify-start gap-2">
        <input
          type="checkbox"
          className="checkbox checkbox-sm"
          checked={v.uppercase}
          disabled={disabled}
          onChange={(e) => onChange({ ...v, uppercase: e.target.checked })}
        />
        Uppercase suffix
      </label>
    </div>
  );
};

export default {
  meta,
  defaultParams,
  Panel,
  normalizeParams,
};
```

### `src/runner.ts`

```ts
function extFromMime(mimeType: string): string {
  if (mimeType === "image/png") return "png";
  if (mimeType === "image/jpeg") return "jpg";
  if (mimeType === "image/webp") return "webp";
  return "bin";
}

function guessMimeType(input: File): string {
  return input.type || "application/octet-stream";
}

export async function run(
  ctx: { inputFile: File; signal: AbortSignal },
  rawParams: Record<string, unknown>,
  helpers?: { reportProgress?: (processed: number, total: number) => void },
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  if (ctx.signal.aborted) throw new Error("Operation aborted");
  helpers?.reportProgress?.(1, 3);

  const suffixRaw = typeof rawParams.suffix === "string" ? rawParams.suffix.trim() : "hello";
  const uppercase = rawParams.uppercase === true;
  const suffix = (uppercase ? suffixRaw.toUpperCase() : suffixRaw) || "hello";

  const bytes = new Uint8Array(await ctx.inputFile.arrayBuffer());
  if (ctx.signal.aborted) throw new Error("Operation aborted");
  helpers?.reportProgress?.(2, 3);

  const mimeType = guessMimeType(ctx.inputFile);
  const base = ctx.inputFile.name.replace(/\.[^.]+$/, "");
  const ext = extFromMime(mimeType);

  helpers?.reportProgress?.(3, 3);
  return {
    bytes,
    mimeType,
    fileName: `${base}-${suffix}.${ext}`,
  };
}

export default { run };
```

## 6) Local Checklist Before Hand-off

- UI module exports match expected contract.
- Runner module returns valid `{ bytes, mimeType, fileName }`.
- Both entries are bundled into `dist/*.mjs`.
- Any wasm sidecar files required by runner are present in `dist/`.
- Plugin runs in Studio panel without runtime import errors.
