/**
 * Formats bytes into a human-readable string (KB, MB, GB, etc.)
 * @param {number} bytes - The number of bytes to format
 * @param {number} [decimals=2] - Number of decimal places to include
 * @returns {string} Formatted string representation of the byte size
 */
export const formatBytes = (bytes: number, decimals: number = 2): string => {
  if (bytes === 0) return "0 Bytes";

  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];

  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
};

/**
 * Formats a duration in milliseconds to a human-readable time string
 * @param {number} ms - Duration in milliseconds
 * @returns {string} Formatted time string
 */
export const formatDuration = (ms: number): string => {
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;

  const minutes = Math.floor(ms / 60000);
  const seconds = ((ms % 60000) / 1000).toFixed(1);
  return `${minutes}m ${seconds}s`;
};

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
