/** Time formatting for the storage surface, so every timestamp reads the same way. */
export function formatClockTime(
  value: string | null | undefined,
  locale: string,
  fallback = "—",
): string {
  if (!value) return fallback;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return fallback;
  return parsed.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" });
}

export function formatDateTime(
  value: string | null | undefined,
  locale: string,
  fallback = "—",
): string {
  if (!value) return fallback;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return fallback;
  return parsed.toLocaleString(locale);
}
