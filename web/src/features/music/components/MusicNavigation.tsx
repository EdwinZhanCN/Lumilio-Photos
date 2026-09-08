import { ChevronLeft, ChevronRight, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { useAuth } from "@/features/auth";
import UserAvatar from "@/components/ui/UserAvatar";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useI18n } from "@/lib/i18n";
import "./MusicLibrary.css";

export default function MusicNavigation() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [params] = useSearchParams();
  const query = params.get("q") ?? "";
  const [search, setSearch] = useState(query);
  useEffect(() => setSearch(query), [query]);
  return (
    <header className="music-chrome">
      <div className="flex gap-2">
        <button
          className="btn btn-ghost btn-circle btn-sm"
          aria-label={t("common.previous", "Previous")}
          onClick={() => void navigate(-1)}
        >
          <ChevronLeft />
        </button>
        <button
          className="btn btn-ghost btn-circle btn-sm"
          aria-label={t("common.next", "Next")}
          onClick={() => void navigate(1)}
        >
          <ChevronRight />
        </button>
      </div>
      <nav aria-label={t("music.title", "Music")}>
        <Link to="/">{t("sidebar.home", "Home")}</Link>
        <Link to="/music" aria-current="page">
          {t("music.browse.title", "Library")}
        </Link>
      </nav>
      <div className="music-nav-tools">
        <form
          className="music-search"
          onSubmit={(event) => {
            event.preventDefault();
            void navigate(
              `/music?view=tracks${search.trim() ? `&q=${encodeURIComponent(search.trim())}` : ""}`,
            );
          }}
        >
          <Search className="size-4" />
          <input
            aria-label={t("music.search.label", "Search music")}
            placeholder={t("music.search.label", "Search music")}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </form>
        <Link to="/settings" aria-label={t("sidebar.settings", "Settings")}>
          <UserAvatar
            name={user?.display_name || user?.username || t("music.title", "Music")}
            assetId={user?.avatar_asset_id}
            size="size-8"
            textSize="text-xs"
          />
        </Link>
      </div>
    </header>
  );
}
