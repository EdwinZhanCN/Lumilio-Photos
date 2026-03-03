# Lumilio Border Plugin (CDN Package Example)

This plugin demonstrates the Studio runtime contract with split entries:

- `entries.ui` -> React parameter panel
- `entries.runner` -> worker-executed processing logic

## Structure

- `manifest.json`: Runtime manifest returned by registry.
- `src/ui.tsx`: Source UI module.
- `src/runner.ts`: Source runner module.
- `dist/ui.mjs`: ESM UI artifact for CDN.
- `dist/runner.mjs`: ESM runner artifact for CDN.
- `dist/border_wasm.js`: wasm-bindgen glue.
- `dist/border_wasm_bg.wasm`: wasm binary.

## Runtime contract

UI module exports:

- `meta`
- `defaultParams`
- `Panel`
- optional `normalizeParams`

Runner module exports:

- `run(ctx, params, helpers)`

Where `ctx` includes:

- `inputFile`
- `signal`
- `manifest`

## Publish paths (example)

- `https://cdn.example.com/plugins/com.lumilio.border/0.2.0/ui.mjs`
- `https://cdn.example.com/plugins/com.lumilio.border/0.2.0/runner.mjs`
- `https://cdn.example.com/plugins/com.lumilio.border/0.2.0/border_wasm.js`
- `https://cdn.example.com/plugins/com.lumilio.border/0.2.0/border_wasm_bg.wasm`
- `https://cdn.example.com/plugins/com.lumilio.border/0.2.0/manifest.json`

## Notes

- `manifest.signature.value` must be replaced with a real signature before publishing.
- `react` and `react-dom` remain peer dependencies to avoid duplicate React runtime.
- Runner output is now always `image/png` to preserve alpha channel.
- Runner fallback is now resilient: decode failures trigger browser-side PNG transcode and retry.
- Heavy modes (`FROSTED`/`VIGNETTE`) auto-downscale very large images (PNG path) before wasm processing to reduce latency and OOM risk.
- For Cloudflare publish, pass `--cdn-origin` so runtime entry URLs are rewritten to your real CDN origin.
