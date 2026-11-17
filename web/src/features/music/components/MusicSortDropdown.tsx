import { ArrowUpDown } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export type MusicSortValue = "" | "title" | "artist" | "album" | "track";

export default function MusicSortDropdown({
  sort,
  onSortChange,
  emptyLabel,
}: {
  sort: MusicSortValue;
  onSortChange: (value: MusicSortValue) => void;
  /** Label for the "" (default/unsorted) option. Defaults to "Recently added"; playlists pass "Playlist order". */
  emptyLabel?: string;
}) {
  const { t } = useI18n();
  const defaultLabel = emptyLabel ?? t("music.tracks.sort.recent", "Recently added");
  const currentSortLabel =
    sort === "title"
      ? t("music.tracks.sort.title", "Title")
      : sort === "artist"
        ? t("music.tracks.sort.artist", "Artist")
        : sort === "album"
          ? t("music.tracks.sort.album", "Album")
          : sort === "track"
            ? t("music.tracks.sort.track", "Track number")
            : defaultLabel;
  return (
    <div className="dropdown dropdown-end">
      <div tabIndex={0} role="button" className="btn btn-sm btn-soft btn-info gap-2">
        <ArrowUpDown className="size-4" />
        {t("music.tracks.sortBy", "Sort by {{sort}}", { sort: currentSortLabel })}
      </div>
      <ul
        tabIndex={0}
        className="dropdown-content menu bg-base-200 rounded-box z-dropdown w-44 p-2 shadow-xl"
      >
        {(
          [
            ["", defaultLabel],
            ["title", t("music.tracks.sort.title", "Title")],
            ["artist", t("music.tracks.sort.artist", "Artist")],
            ["album", t("music.tracks.sort.album", "Album")],
            ["track", t("music.tracks.sort.track", "Track number")],
          ] as const
        ).map(([value, label]) => (
          <li key={value}>
            <button
              onClick={() => onSortChange(value as MusicSortValue)}
              className={sort === value ? "active" : ""}
            >
              {label}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
