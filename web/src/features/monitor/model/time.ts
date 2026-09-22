import type { TFunction } from "i18next";

/**
 * The single time formatter for Server Monitor.
 *
 * Timestamps arrive as RFC 3339 strings and durations as epoch milliseconds
 * from several endpoints. Every Monitor surface resolves them here so relative
 * wording, absolute formatting, and missing-value behavior cannot drift between
 * tabs. `now` is injectable so the relative branch is testable without a clock.
 */

/** Compact duration for averages. Null while the server reports no sample. */
export function formatDuration(ms?: number | null): string | null {
  if (ms == null) return null;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)}s`;
  if (ms < 3_600_000) return `${(ms / 60_000).toFixed(ms < 600_000 ? 1 : 0)}m`;
  return `${(ms / 3_600_000).toFixed(1)}h`;
}

/**
 * Relative age resolved through i18n keys. A future timestamp clamps to
 * "just now" instead of rendering a negative age.
 */
export function formatRelativeTime(
  timestamp: string | undefined,
  t: TFunction,
  language?: string,
  now: number = Date.now(),
): string {
  if (!timestamp) return t("monitor.queueSummary.time.never");

  const date = new Date(timestamp);
  const secondsAgo = Math.max(0, Math.floor((now - date.getTime()) / 1000));

  if (secondsAgo < 60) {
    return t("monitor.queueSummary.time.justNow");
  }
  if (secondsAgo < 3600) {
    return t("monitor.queueSummary.time.minutesAgo", {
      count: Math.floor(secondsAgo / 60),
    });
  }
  if (secondsAgo < 86_400) {
    return t("monitor.queueSummary.time.hoursAgo", {
      count: Math.floor(secondsAgo / 3600),
    });
  }
  if (secondsAgo < 604_800) {
    return t("monitor.queueSummary.time.daysAgo", {
      count: Math.floor(secondsAgo / 86_400),
    });
  }
  return date.toLocaleDateString(language);
}

/**
 * Absolute timestamp for diagnostics. Null when the value is missing or
 * unparseable, so each caller keeps its own placeholder instead of inventing a
 * shared one. `dateStyle`/`timeStyle` stay per-surface because a support table
 * and a capacity line legitimately read differently.
 */
export function formatAbsoluteTime(
  value: string | undefined,
  options: {
    language?: string;
    dateStyle?: "short" | "medium";
    timeStyle?: "short" | "medium";
  } = {},
): string | null {
  if (!value) return null;

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;

  return new Intl.DateTimeFormat(options.language, {
    dateStyle: options.dateStyle ?? "short",
    timeStyle: options.timeStyle ?? "short",
  }).format(parsed);
}

/**
 * Diagnostic timestamp: a placeholder when the field is absent, but the raw
 * value when the Server sent something unparseable. An operator reading a
 * support view needs to see what actually arrived.
 */
export function formatDiagnosticTime(
  value: string | undefined,
  fallback: string,
  options: {
    language?: string;
    dateStyle?: "short" | "medium";
    timeStyle?: "short" | "medium";
  } = {},
): string {
  if (!value) return fallback;
  return formatAbsoluteTime(value, options) ?? value;
}
