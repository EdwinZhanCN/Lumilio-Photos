/**
 * Converts a capture offset (minutes) to a fixed-offset IANA-style timezone string.
 * @example offsetToTzString(480) → "+08:00"
 *          offsetToTzString(-300) → "-05:00"
 */
const offsetToTzString = (offsetMinutes: number): string => {
  const sign = offsetMinutes < 0 ? "-" : "+";
  const abs = Math.abs(offsetMinutes);
  const hh = String(Math.floor(abs / 60)).padStart(2, "0");
  const mm = String(abs % 60).padStart(2, "0");
  return `${sign}${hh}:${mm}`;
};

/**
 * Formats a UTC ISO timestamp string to a locale-aware date/time string,
 * adjusted to the original capture timezone when capture_offset_minutes is provided.
 * Falls back to the browser's local timezone when the offset is absent.
 *
 * @param isoString    - UTC ISO 8601 timestamp (e.g. "2023-01-01T12:00:00Z")
 * @param offsetMinutes - capture_offset_minutes from asset metadata
 * @param locale       - optional locale override (defaults to browser locale)
 */
export const formatCaptureTime = (
  isoString: string | undefined | null,
  offsetMinutes?: number | null,
  locale?: string,
): string => {
  if (!isoString) return "";
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return isoString;

  if (typeof offsetMinutes === "number") {
    const tz = offsetToTzString(offsetMinutes);
    return date.toLocaleString(locale, { timeZone: tz });
  }

  return date.toLocaleString(locale);
};
