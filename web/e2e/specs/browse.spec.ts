import { test } from "../fixtures/test";

/**
 * Core browsing paths that ADR-006 assigns to E2E rather than the Vitest
 * integration project: they exercise the full AssetBrowser — the WASM justified
 * layout, viewport virtualization, URL/route state and real selection — which is
 * only honest against the real gallery and first-party API.
 *
 * Placeholders (`test.fixme`) document the paths to implement; they are skipped,
 * stay out of the `@smoke` gate, and never fail CI until written. Replace each
 * body with a real flow using LoginPage + GalleryPage (see scan.spec.ts), then
 * add `@smoke` only if the path belongs in the fast gate.
 *
 * Superseded integration tests (removed in the browser-mode migration):
 *   - AlbumDetailsFlow.test.tsx  → album bulk-remove path below
 *   - PhotoPicker.test.tsx       → picker selection path below
 *     (its pure stack→representative logic stays unit-covered in
 *      features/assets/model/browseItems.test.ts)
 */

test.fixme(
  "removing selected assets from an album issues per-asset DELETEs and refreshes",
  async () => {
    // TODO: sign in, open an album with >=2 assets, enter selection mode, select
    // two, run "Remove from this album", confirm, and assert both are gone and
    // the originals remain in the library (album membership only).
  },
);

test.fixme("the photo picker returns the representative asset of a chosen stack", async () => {
  // TODO: open a picker (e.g. user avatar), the gallery is locked to PHOTO, pick
  // a stacked item, and assert onSelect resolves to the stack's representative
  // asset id (the cover), not a member.
});
