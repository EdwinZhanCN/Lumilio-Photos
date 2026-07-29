import { describe, expect, it } from "vitest";
import { eventDateRange, eventTitle, type EventSummary } from "./event";

const event = {
  event_id: "550e8400-e29b-41d4-a716-446655440000",
  start_at: Date.UTC(2026, 0, 2, 12) * 1000,
  end_at: Date.UTC(2026, 0, 2, 14) * 1000,
  is_hidden: false,
  media_count: 2,
  displayable_count: 2,
} satisfies EventSummary;

describe("Event presentation", () => {
  it("uses a localized fallback without persisting generated prose", () => {
    expect(eventTitle(event, (_key, fallback) => fallback)).toBe("Event");
    expect(eventTitle({ ...event, title_override: "  Walk  " }, (_key, fallback) => fallback)).toBe(
      "Walk",
    );
  });

  it("formats same-day ranges once", () => {
    expect(eventDateRange(event, "en-US")).toBe("Jan 2, 2026");
  });
});
