import type { AgentRefFacetsDTO } from "./types";

/** Full grouped count, e.g. "24,890". The handoff renders counts in full with
 * tabular-nums rather than compact notation. */
export function fmt(value: number | undefined, locale?: string): string {
  return (value ?? 0).toLocaleString(locale);
}

function year(iso?: string): string {
  return iso ? String(iso).slice(0, 4) : "";
}

/** Year-span label for a date range: "2023" or "2023–2025". Empty when absent. */
export function dateRangeLabel(range: AgentRefFacetsDTO["date_range"] | undefined): string {
  if (!range?.from || !range?.to) return "";
  const a = year(range.from);
  const b = year(range.to);
  if (!a || !b) return "";
  return a === b ? a : `${a}–${b}`;
}

const IMAGE_KEYS = new Set(["image", "photo"]);

/** Sum the type-count facet for a kind, tolerating backend ("PHOTO"/"VIDEO"),
 * mock ("photo") and handoff ("IMAGE") casing. */
export function typeCount(
  types: AgentRefFacetsDTO["types"] | undefined,
  kind: "image" | "video",
): number {
  if (!types) return 0;
  let total = 0;
  for (const [key, value] of Object.entries(types)) {
    const k = key.toLowerCase();
    if (kind === "image" ? IMAGE_KEYS.has(k) : k === "video") total += value ?? 0;
  }
  return total;
}
