import { Heart } from "lucide-react";
import { useI18n } from "@/lib/i18n";

type MusicFilterBarProps = {
  scope: "tracks" | "catalog";
  active: boolean;
  onChange: (active: boolean) => void;
};

/** Compact binary filter with one visual treatment across Music browse views. */
export default function MusicFilterBar({ scope, active, onChange }: MusicFilterBarProps) {
  const { t } = useI18n();
  const label =
    scope === "tracks"
      ? t("music.tracks.likedOnly", "Liked")
      : t("music.favorites.label", "Favorites");
  const buttonClass =
    "inline-flex h-9 items-center gap-1.5 rounded-lg px-3 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-primary";

  return (
    <div
      role="group"
      aria-label={t("music.filters.label", "Music filters")}
      className="inline-flex rounded-xl bg-base-200/70 p-1"
    >
      <button
        type="button"
        aria-pressed={!active}
        className={`${buttonClass} ${!active ? "bg-base-100 text-base-content shadow-sm" : "text-base-content/60 hover:bg-base-100/70 hover:text-base-content"}`}
        onClick={() => onChange(false)}
      >
        {t("music.filters.all", "All")}
      </button>
      <button
        type="button"
        aria-pressed={active}
        className={`${buttonClass} ${active ? "bg-base-100 text-base-content shadow-sm" : "text-base-content/60 hover:bg-base-100/70 hover:text-base-content"}`}
        onClick={() => onChange(true)}
      >
        <Heart className="size-4" fill={active ? "currentColor" : "none"} />
        {label}
      </button>
    </div>
  );
}
