/**
 * Processing tray scale.
 *
 * The Processing tray animates a fixed reference: eighteen layers at full, and
 * the asset's own four tiers (`Full` → `Two Thirds` → `One Third` → `Empty`).
 * The driven `progress` property is a completion percentage where 100 is an
 * empty tray, so a shrinking backlog drains the tray instead of re-scaling it.
 *
 * The reference is a product constant, never the current maximum. Recomputing it
 * per poll would leave the tray looking full while real work disappeared, and
 * the Server exposes no batch total to divide by — inventing one is forbidden.
 * Layer units exist only to compress a count into a readable stack; the exact
 * number always stays visible as text.
 */

/** Layers in the asset's `Full` timeline. */
export const TRAY_LAYERS = 18;

/** How much pending work one layer stands for, per work type. */
export const LAYER_UNITS = {
  assets: 32,
  repositories: 1,
  projections: 2,
  operations: 1,
} as const;

export type WorkType = keyof typeof LAYER_UNITS;

/** The tray's four discrete visual states, in the asset's own order. */
export type TrayTier = "full" | "twoThirds" | "oneThird" | "empty";

/** Pending work that fills the tray for a work type. */
export function trayReference(type: WorkType): number {
  return LAYER_UNITS[type] * TRAY_LAYERS;
}

/**
 * Completion percentage for a pending count, clamped to the asset's 0–100
 * range. `0` is a full tray and `100` an empty one.
 */
export function trayProgress(pending: number, type: WorkType): number {
  const reference = trayReference(type);
  const safePending = Number.isFinite(pending) ? Math.max(0, pending) : 0;
  return Math.min(100, Math.max(0, 100 * (1 - safePending / reference)));
}

/**
 * Layers the tray still shows for a pending count, rounded up so work is never
 * hidden and capped at the asset's full stack. Layers track outstanding work;
 * {@link trayTier} tracks completion. The two are views of the same number and
 * must agree — a tier is exactly a band of layer counts.
 */
export function trayLayers(pending: number, type: WorkType): number {
  const safePending = Number.isFinite(pending) ? Math.max(0, pending) : 0;
  return Math.min(TRAY_LAYERS, Math.ceil(safePending / LAYER_UNITS[type]));
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
