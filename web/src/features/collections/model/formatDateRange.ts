/**
 * Formats an optional [start, end] pair of ISO date strings for folder/tag
 * summary cards. Uses the browser locale and returns "" when both ends are
 * missing so callers can drop it from a joined subtitle.
 */
export function formatDateRange(start?: string, end?: string): string {
  if (!start && !end) return "";

  const format = (value: string) =>
    new Date(value).toLocaleDateString(undefined, { year: "numeric", month: "short" });

  if (start && end) {
    const startLabel = format(start);
    const endLabel = format(end);
    return startLabel === endLabel ? startLabel : `${startLabel} – ${endLabel}`;
  }

  return format(start ?? end ?? "");
}
