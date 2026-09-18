import { RuntimeLoader } from "@rive-app/react-canvas";
import wasmUrl from "@rive-app/canvas/rive.wasm?url";

/**
 * Rive runtime wiring for the Processing tray.
 *
 * Two things happen here and nowhere else:
 *
 * 1. The runtime WASM is served from our own bundle. Left alone, the runtime
 *    pulls it from a public CDN, which a local-first app must not depend on at
 *    runtime or in CI, so the fallback URL is disabled outright rather than
 *    merely overridden.
 * 2. The asset is imported with `import.meta.glob(..., { query: "?url" })`, the
 *    pattern this repository already uses for internal assets, so the bundler
 *    hashes and emits it instead of it being hand-copied into `public/`.
 */

const ASSET_URLS = import.meta.glob<string>("../../../../assets/rive/*.riv", {
  eager: true,
  query: "?url",
  import: "default",
});

/** Artboard and state machine as authored. A rename in the asset must fail loudly. */
export const PROCESSING_ARTBOARD = "File Processing — Golden Inbox";
export const PROCESSING_STATE_MACHINE = "File Processing";

/** Resolved URL of a bundled Processing asset, or undefined when it is missing. */
export function processingAssetUrl(name = "golden-inbox.riv"): string | undefined {
  return Object.entries(ASSET_URLS).find(([path]) => path.endsWith(`/${name}`))?.[1];
}

// Configured at module scope: the runtime must know where its WASM lives before
// the first Rive instance loads, and every consumer of this module loads one.
RuntimeLoader.setWasmUrl(wasmUrl);
RuntimeLoader.setWasmFallbackUrl(null);
