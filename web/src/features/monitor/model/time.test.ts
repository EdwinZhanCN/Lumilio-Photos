import { describe, expect, it } from "vite-plus/test";
import { translateDouble } from "@test/translateDouble";
import {
  formatAbsoluteTime,
  formatDiagnosticTime,
  formatDuration,
  formatRelativeTime,
} from "./time";

const NOW = Date.parse("2026-06-12T12:00:00.000Z");

function ago(ms: number): string {
  return new Date(NOW - ms).toISOString();
}

describe("formatDuration", () => {
  it("returns null while the server reports no sample", () => {
    expect(formatDuration(undefined)).toBeNull();
    expect(formatDuration(null)).toBeNull();
  });

  it("switches unit and precision at each boundary", () => {
    expect(formatDuration(0)).toBe("0ms");
    expect(formatDuration(999)).toBe("999ms");
    expect(formatDuration(1000)).toBe("1.0s");
    expect(formatDuration(9_999)).toBe("10.0s");
    expect(formatDuration(10_000)).toBe("10s");
    expect(formatDuration(59_999)).toBe("60s");
    expect(formatDuration(60_000)).toBe("1.0m");
    expect(formatDuration(599_999)).toBe("10.0m");
    expect(formatDuration(600_000)).toBe("10m");
    expect(formatDuration(3_600_000)).toBe("1.0h");
    expect(formatDuration(5_400_000)).toBe("1.5h");
  });
});

describe("formatRelativeTime", () => {
  const { t } = translateDouble();

  it("resolves each age bucket to its own key", () => {
    expect(formatRelativeTime(undefined, t, "en", NOW)).toBe("monitor.queueSummary.time.never");
    expect(formatRelativeTime(ago(30_000), t, "en", NOW)).toBe("monitor.queueSummary.time.justNow");
    expect(formatRelativeTime(ago(5 * 60_000), t, "en", NOW)).toBe(
      "monitor.queueSummary.time.minutesAgo",
    );
    expect(formatRelativeTime(ago(5 * 3_600_000), t, "en", NOW)).toBe(
      "monitor.queueSummary.time.hoursAgo",
    );
    expect(formatRelativeTime(ago(5 * 86_400_000), t, "en", NOW)).toBe(
      "monitor.queueSummary.time.daysAgo",
    );
  });

  it("passes the bucket count rather than pre-formatting it", () => {
    const { t: stub, calls } = translateDouble();
    formatRelativeTime(ago(5 * 60_000), stub, "en", NOW);
    expect(calls).toEqual([{ key: "monitor.queueSummary.time.minutesAgo", options: { count: 5 } }]);
  });

  it("clamps a future timestamp instead of rendering a negative age", () => {
    const { t: stub, calls } = translateDouble();
    expect(formatRelativeTime(ago(-60_000), stub, "en", NOW)).toBe(
      "monitor.queueSummary.time.justNow",
    );
    expect(calls.map((call) => call.key)).toEqual(["monitor.queueSummary.time.justNow"]);
  });

  it("falls back to a locale date beyond a week", () => {
    const timestamp = ago(8 * 86_400_000);
    expect(formatRelativeTime(timestamp, t, "en", NOW)).toBe(
      new Date(timestamp).toLocaleDateString("en"),
    );
  });
});

describe("formatAbsoluteTime", () => {
  it("returns null when the value is absent or unparseable", () => {
    expect(formatAbsoluteTime(undefined)).toBeNull();
    expect(formatAbsoluteTime("")).toBeNull();
    expect(formatAbsoluteTime("not-a-date")).toBeNull();
  });

  it("formats a valid timestamp", () => {
    expect(
      formatAbsoluteTime("2026-06-12T12:00:00Z", {
        language: "en",
        dateStyle: "medium",
        timeStyle: "medium",
      }),
    ).toContain("2026");
  });
});

describe("formatDiagnosticTime", () => {
  it("uses the caller's placeholder when the field is absent", () => {
    expect(formatDiagnosticTime(undefined, "n/a")).toBe("n/a");
  });

  it("shows the raw value instead of hiding an unparseable timestamp", () => {
    expect(formatDiagnosticTime("garbage", "n/a")).toBe("garbage");
  });
});
