/** Entity kinds the user can @-mention in the agent input. */
export type MentionType = "person" | "album" | "pin" | "camera" | "lens";

/** A single mention candidate shown in the picker. */
export interface MentionEntity {
  id: string;
  label: string;
  type: MentionType;
}

export interface MentionPayload {
  type: MentionType;
  id: string;
  label: string;
}

export interface MentionSource {
  type: MentionType;
  search: (query: string) => MentionEntity[];
}

function fuzzyMatch(query: string, label: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return label.toLowerCase().includes(q);
}

export function createMentionSources(data: {
  people: { person_id: number; name?: string | null }[];
  albums: { album_id: number; album_name: string }[];
  pins: { pin_id: string; title?: string | null; summary?: string | null }[];
  cameras: string[];
  lenses: string[];
}): MentionSource[] {
  return [
    {
      type: "person",
      search: (q) =>
        data.people
          .filter((p) => p.name && fuzzyMatch(q, p.name))
          .slice(0, 20)
          .map((p) => ({
            id: String(p.person_id),
            label: p.name ?? String(p.person_id),
            type: "person" as const,
          })),
    },
    {
      type: "album",
      search: (q) =>
        data.albums
          .filter((a) => fuzzyMatch(q, a.album_name))
          .slice(0, 20)
          .map((a) => ({
            id: String(a.album_id),
            label: a.album_name,
            type: "album" as const,
          })),
    },
    {
      type: "pin",
      search: (q) =>
        data.pins
          .filter((p) => {
            const label = p.title ?? p.summary ?? p.pin_id;
            return fuzzyMatch(q, label);
          })
          .slice(0, 20)
          .map((p) => ({
            id: p.pin_id,
            label: p.title ?? p.summary ?? p.pin_id,
            type: "pin" as const,
          })),
    },
    {
      type: "camera",
      search: (q) =>
        data.cameras
          .filter((name) => fuzzyMatch(q, name))
          .slice(0, 20)
          .map((name) => ({
            id: name,
            label: name,
            type: "camera" as const,
          })),
    },
    {
      type: "lens",
      search: (q) =>
        data.lenses
          .filter((name) => fuzzyMatch(q, name))
          .slice(0, 20)
          .map((name) => ({
            id: name,
            label: name,
            type: "lens" as const,
          })),
    },
  ];
}

/** All entities of one mention type (RichInput SELECT_ENTITY phase). */
export function getEntitiesForType(
  sources: MentionSource[],
  type: MentionType,
  query = "",
): MentionEntity[] {
  const source = sources.find((s) => s.type === type);
  return source?.search(query) ?? [];
}

/** Flat search across all mention types, ranked by source order. */
export function searchAllMentions(sources: MentionSource[], query: string): MentionEntity[] {
  return sources.flatMap((s) => s.search(query)).slice(0, 30);
}
