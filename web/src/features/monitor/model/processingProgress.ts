/**
 * Processing tray scale.
 *
 * The Processing tab animates exactly one tray: files awaiting processing.
 * Every other kind of work is a list row with exact counts, so the Rive asset
 * never has to invent a visual unit for a Repository scan or a projection.
 *
 * The tray animates a fixed reference: eighteen layers at full, and the
 * asset's own four tiers (`Full` → `Two Thirds` → `One Third` → `Empty`). The
 * driven `progress` property is a completion percentage where 100 is an empty
 * tray, so a shrinking backlog drains the tray instead of re-scaling it.
 *
 * The reference is a product constant, never the current maximum. Recomputing it
 * per poll would leave the tray looking full while real work disappeared, and
 * the Server exposes no batch total to divide by — inventing one is forbidden.
 * Layer units exist only to compress a count into a readable stack; the exact
 * number always stays visible as text.
 */

/** Layers in the asset's `Full` timeline. */
export const TRAY_LAYERS = 18;

/** Pending files one layer stands for. */
export const FILES_PER_LAYER = 32;

/** Pending files that fill the tray. */
export const TRAY_REFERENCE = FILES_PER_LAYER * TRAY_LAYERS;

/** The tray's four discrete visual states, in the asset's own order. */
export type TrayTier = "full" | "twoThirds" | "oneThird" | "empty";

function safePending(pending: number): number {
  return Number.isFinite(pending) ? Math.max(0, pending) : 0;
}

/**
 * Completion percentage for a pending count, clamped to the asset's 0–100
 * range. `0` is a full tray and `100` an empty one.
 */
export function trayProgress(pending: number): number {
  return Math.min(100, Math.max(0, 100 * (1 - safePending(pending) / TRAY_REFERENCE)));
}

/**
 * Layers the tray still shows for a pending count, rounded up so work is never
 * hidden and capped at the asset's full stack. Layers track outstanding work;
 * {@link trayTier} tracks completion. The two are views of the same number and
 * must agree — a tier is exactly a band of layer counts.
 */
export function trayLayers(pending: number): number {
  return Math.min(TRAY_LAYERS, Math.ceil(safePending(pending) / FILES_PER_LAYER));
}

/**
 * The tier the asset renders for a completion percentage. Thresholds are the
 * asset's own: below a third is `Full`, below two thirds `Two Thirds`, below
 * complete `One Third`, and complete `Empty`.
 */
export function trayTier(progress: number): TrayTier {
  if (progress >= 100) return "empty";
  if (progress >= 200 / 3) return "oneThird";
  if (progress >= 100 / 3) return "twoThirds";
  return "full";
}
