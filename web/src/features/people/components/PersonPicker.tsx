import { useMemo, useState } from "react";
import { Check, Search, UserRound } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import { usePeople } from "../hooks/usePeople";
import type { PersonSummaryList } from "../people.types";

type PersonPickerProps = {
  /** Person IDs to exclude from the list (e.g. the target/self). */
  excludeIds?: number[];
  /** Currently selected person IDs. */
  selectedIds: number[];
  onChange: (next: number[]) => void;
  /** Allow selecting more than one person. Defaults to single-select. */
  multiSelect?: boolean;
  repositoryId?: string;
  /** Include hidden people in the searchable list. */
  includeHidden?: boolean;
};

/**
 * Searchable people list with single- or multi-select, reused by the merge and
 * move-face correction flows so both share one picker mental model. Hidden
 * people are excluded unless `includeHidden` is set.
 */
export default function PersonPicker({
  excludeIds = [],
  selectedIds,
  onChange,
  multiSelect = false,
  repositoryId,
  includeHidden = false,
}: PersonPickerProps) {
  const { t } = useI18n();
  const [search, setSearch] = useState("");
  const { people, isLoading } = usePeople({
    repositoryId,
    includeHidden,
    limit: 100,
  });

  const excluded = useMemo(() => new Set(excludeIds), [excludeIds]);
  const filtered = useMemo<PersonSummaryList>(() => {
    const query = search.trim().toLowerCase();
    return people.filter((person) => {
      if (excluded.has(person.person_id ?? -1)) return false;
      if (!query) return true;
      return (person.name ?? "").toLowerCase().includes(query);
    });
  }, [people, excluded, search]);

  const toggle = (personId: number) => {
    if (multiSelect) {
      onChange(
        selectedIds.includes(personId)
          ? selectedIds.filter((id) => id !== personId)
          : [...selectedIds, personId],
      );
    } else {
      onChange(selectedIds.includes(personId) ? [] : [personId]);
    }
  };

  return (
    <div className="space-y-3">
      <label className="input input-bordered flex items-center gap-2">
        <Search className="size-4 text-base-content/50" />
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("people.picker.searchPlaceholder", "Search people")}
          className="grow"
        />
      </label>

      <div className="max-h-72 space-y-1 overflow-y-auto">
        {isLoading ? (
          <div className="py-6 text-center text-sm text-base-content/50">
            {t("common.loading", "Loading…")}
          </div>
        ) : filtered.length === 0 ? (
          <div className="py-6 text-center text-sm text-base-content/50">
            {t("people.picker.empty", "No matching people")}
          </div>
        ) : (
          filtered.map((person) => {
            const personId = person.person_id ?? 0;
            const selected = selectedIds.includes(personId);
            const coverUrl = person.cover_face_image_path
              ? assetUrls.getPersonCoverUrl(personId, repositoryId)
              : null;
            return (
              <button
                key={personId}
                type="button"
                onClick={() => toggle(personId)}
                className={`flex w-full items-center gap-3 rounded-xl border px-3 py-2 text-left transition ${
                  selected ? "border-primary bg-primary/10" : "border-transparent hover:bg-base-200"
                }`}
              >
                <span className="size-10 shrink-0 overflow-hidden rounded-full bg-base-200">
                  {coverUrl ? (
                    <img
                      src={coverUrl}
                      alt={person.name || t("people.unnamed")}
                      className="size-full object-cover"
                    />
                  ) : (
                    <span className="flex size-full items-center justify-center">
                      <UserRound className="size-5 text-base-content/40" />
                    </span>
                  )}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">
                    {person.name || t("people.unnamed")}
                  </span>
                  <span className="block text-xs text-base-content/55">
                    {t("people.photosCount", { count: person.asset_count ?? 0 })}
                  </span>
                </span>
                {selected && <Check className="size-4 shrink-0 text-primary" />}
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}
