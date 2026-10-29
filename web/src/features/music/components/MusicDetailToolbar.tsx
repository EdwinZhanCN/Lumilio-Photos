import { useI18n } from "@/lib/i18n";
import { Pencil, Search, X } from "lucide-react";
import MusicMoreMenu from "./MusicMoreMenu";
import MusicSortDropdown, { type MusicSortValue } from "./MusicSortDropdown";
import MusicFavoriteButton from "./MusicFavoriteButton";

type DetailFavorite = {
  kind: "album" | "artist";
  id?: string;
  favorite?: boolean;
  revision?: number;
};

type MusicDetailToolbarProps = {
  editLabel: string;
  onEdit: () => void;
  moreActions?: Array<{ label: string; onSelect: () => void; danger?: boolean }>;
  sort: MusicSortValue;
  onSortChange: (value: MusicSortValue) => void;
  favorite?: DetailFavorite;
  /** Label for the "" (default/unsorted) option — playlists pass "Playlist order". */
  sortEmptyLabel?: string;
  /** Persistent in-toolbar search (playlists filter their entries from here). */
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder: string;
    ariaLabel: string;
  };
};

/**
 * Shared T1 toolbar for the Album / Playlist / Artist detail pages:
 * Edit · More (ellipsis) · Sort (track-list sort dropdown) · Favorite.
 * `moreActions` and `favorite` are optional and only render when provided.
 */
export default function MusicDetailToolbar({
  editLabel,
  onEdit,
  moreActions,
  sort,
  onSortChange,
  favorite,
  sortEmptyLabel,
  search,
}: MusicDetailToolbarProps) {
  const { t } = useI18n();
  return (
    <div className="mb-6 flex flex-wrap items-center justify-end gap-2">
      <button type="button" className="btn btn-ghost btn-sm" onClick={onEdit}>
        <Pencil className="size-4" />
        {editLabel}
      </button>
      {moreActions && moreActions.length > 0 && <MusicMoreMenu actions={moreActions} />}
      <MusicSortDropdown sort={sort} onSortChange={onSortChange} emptyLabel={sortEmptyLabel} />
      {favorite && <MusicFavoriteButton {...favorite} />}
      {search && (
        <label className="input input-bordered input-sm flex w-full min-w-0 items-center gap-2 sm:w-56">
          <Search className="size-4 text-base-content/50" />
          <input
            type="text"
            className="min-w-0 grow"
            value={search.value}
            onChange={(event) => search.onChange(event.target.value)}
            placeholder={search.placeholder}
            aria-label={search.ariaLabel}
          />
          {search.value && (
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={() => search.onChange("")}
              aria-label={t("music.search.clear", "Clear search")}
            >
              <X className="size-3.5" />
            </button>
          )}
        </label>
      )}
    </div>
  );
}
