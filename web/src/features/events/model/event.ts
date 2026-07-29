import type { components } from "@/lib/http-commons";

export type EventSummary = components["schemas"]["dto.EventSummaryDTO"];
export type EventDetail = components["schemas"]["dto.EventDetailDTO"];
export type EventPatch = components["schemas"]["dto.EventPatchRequestDTO"];
export type EventShareRequest = components["schemas"]["dto.EventShareRequestDTO"];

type Translate = (key: string, fallback: string) => string;

export function eventTitle(event: EventSummary, t: Translate): string {
  const override = event.title_override?.trim();
  if (override) return override;
  return t("events.generatedTitle", "Event");
}

export function eventDateRange(event: EventSummary, locale?: string): string {
  const start = new Date((event.start_at ?? 0) / 1000);
  const end = new Date((event.end_at ?? event.start_at ?? 0) / 1000);
  const formatter = new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
  if (start.toDateString() === end.toDateString()) return formatter.format(start);
  return `${formatter.format(start)} – ${formatter.format(end)}`;
}
