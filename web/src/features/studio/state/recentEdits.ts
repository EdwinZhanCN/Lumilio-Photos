export const STUDIO_RECENT_EDITS_KEY = "lumilio.studio.recent_edits.v1";
const MAX_RECENT = 12;

export interface RecentEditRecord {
  assetId: string;
  name: string;
  width: number | null;
  height: number | null;
  editedAt: string;
}

function parse(raw: string | null): RecentEditRecord[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is RecentEditRecord => {
      if (typeof item !== "object" || item === null) return false;
      const r = item as Record<string, unknown>;
      return typeof r.assetId === "string" && typeof r.editedAt === "string";
    });
  } catch {
    return [];
  }
}

export function readRecentEdits(): RecentEditRecord[] {
  return parse(localStorage.getItem(STUDIO_RECENT_EDITS_KEY)).sort((a, b) =>
    b.editedAt.localeCompare(a.editedAt),
  );
}

export function recordRecentEdit(record: Omit<RecentEditRecord, "editedAt">): RecentEditRecord[] {
  const existing = readRecentEdits().filter((item) => item.assetId !== record.assetId);
  const next: RecentEditRecord[] = [
    { ...record, editedAt: new Date().toISOString() },
    ...existing,
  ].slice(0, MAX_RECENT);
  localStorage.setItem(STUDIO_RECENT_EDITS_KEY, JSON.stringify(next));
  return next;
}

export function clearRecentEdits(): void {
  localStorage.removeItem(STUDIO_RECENT_EDITS_KEY);
}

/** Compact relative-time label, e.g. "2 minutes ago", "Yesterday". */
export function formatRelativeTime(iso: string, locale = "en"): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const now = Date.now();
  const diffMs = now - then;
  const sec = Math.round(diffMs / 1000);
  const min = Math.round(sec / 60);
  const hr = Math.round(min / 60);
  const day = Math.round(hr / 24);

  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });

  if (sec < 45) return rtf.format(0, "second");
  if (min < 60) return rtf.format(-min, "minute");
  if (hr < 24) return rtf.format(-hr, "hour");
  if (day < 7) return rtf.format(-day, "day");
  return new Date(iso).toLocaleDateString(locale);
}
