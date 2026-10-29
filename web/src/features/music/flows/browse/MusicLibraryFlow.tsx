import MusicLyricExcerpt from "../../components/MusicLyricExcerpt";
import MusicCoverCard from "../../components/MusicCoverCard";
import MusicPlaylistAction from "../../components/MusicPlaylistAction";
import Link from "../../components/MusicLink";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Disc3, ListMusic, Mic2, Music2, Play, Search } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router-dom";
import EmptyState from "@/components/ui/EmptyState";
import "../../components/MusicLibrary.css";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import PageHeader from "@/components/ui/PageHeader";
import { useI18n } from "@/lib/i18n";
import {
  useMusicAlbums,
  useMusicArtists,
  useMusicPlaylists,
  useMusicTracks,
} from "../../api/useMusic";
import MusicArtwork from "../../components/MusicArtwork";
import MusicTrackRow from "../../components/MusicTrackRow";
import MusicTrackList from "../../components/MusicTrackList";
import MusicSortDropdown from "../../components/MusicSortDropdown";
import type {
  MusicAlbum,
  MusicArtist,
  MusicPlaybackSource,
  MusicPlaylist,
  MusicTrack,
  MusicView,
} from "../../model/music";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";

const views: Array<{ id: MusicView; icon: typeof Music2 }> = [
  { id: "playlists", icon: ListMusic },
  { id: "albums", icon: Disc3 },
  { id: "artists", icon: Mic2 },
  { id: "tracks", icon: ListMusic },
];

function readView(value: string | null): MusicView {
  return views.some((item) => item.id === value) ? (value as MusicView) : "playlists";
}

function sourceFor(
  query: string,
  likedOnly: boolean,
  sort: MusicPlaybackSource["sort"],
): MusicPlaybackSource {
  return { kind: "query", query, liked_only: likedOnly, sort };
}

export default function MusicLibraryFlow() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { playSource } = useMusicPlayer();
  const view = readView(searchParams.get("view"));
  const query = searchParams.get("q") ?? "";
  const sort = (searchParams.get("sort") ?? "") as "" | "title" | "artist" | "album" | "track";
  const favoritesOnly = searchParams.get("favorites") === "1";
  const likedOnly = searchParams.get("liked") === "1";
  const offsetValue = Number(searchParams.get("offset") ?? 0);
  const offset = Number.isSafeInteger(offsetValue) && offsetValue >= 0 ? offsetValue : 0;
  const showHighlights = !likedOnly;
  const scroll = useRef<HTMLDivElement>(null);
  const stickOnViewChange = useRef(false);
  const [search, setSearch] = useState(query);
  useEffect(() => setSearch(query), [query]);
  useBreadcrumbs([{ label: t("sidebar.home", "Home"), to: "/" }, { label: t("music.title") }]);

  // After a tab-driven view change, wait until the new pane has been
  // committed/laid out (double rAF), then scroll the sticky tab bar to the
  // container top (clamped to the scrollable range). Only runs for tab clicks
  // (see onClick). Measuring after render avoids the jitter/clamp that happens
  // when the target is computed against the pre-switch layout.
  useEffect(() => {
    if (!stickOnViewChange.current) return;
    stickOnViewChange.current = false;
    let raf2 = 0;
    const measure = () => {
      const container = scroll.current;
      const bar = container?.querySelector<HTMLElement>(".sticky");
      if (!container || !bar) return;
      const target =
        container.scrollTop +
        Math.round(bar.getBoundingClientRect().top - container.getBoundingClientRect().top);
      const max = Math.max(0, container.scrollHeight - container.clientHeight);
      container.scrollTo({ top: Math.max(0, Math.min(target, max)), behavior: "smooth" });
    };
    const raf1 = requestAnimationFrame(() => {
      raf2 = requestAnimationFrame(measure);
    });
    return () => {
      cancelAnimationFrame(raf1);
      cancelAnimationFrame(raf2);
    };
  }, [view]);

  const tracksQuery = useMusicTracks({
    query,
    sort,
    likedOnly,
    limit: 50,
    offset,
    enabled: view === "tracks",
  });
  const highlightQuery = useMusicTracks({ limit: 12, likedOnly: true, enabled: showHighlights });
  const likedQuery = useMusicTracks({ limit: 1, likedOnly: true, enabled: showHighlights });
  const albumsQuery = useMusicAlbums({
    query,
    favoritesOnly,
    limit: view === "albums" ? 50 : 10,
    offset: view === "albums" ? offset : 0,
    enabled: view === "overview" || view === "albums",
  });
  const artistsQuery = useMusicArtists({
    query,
    favoritesOnly,
    limit: 50,
    offset,
    enabled: view === "artists",
  });
  const playlistsQuery = useMusicPlaylists({
    limit: 50,
    offset,
    enabled: view === "playlists",
  });

  const tracks = tracksQuery.data?.items ?? [];
  const albums = albumsQuery.data?.items ?? [];
  const artists = artistsQuery.data?.items ?? [];
  const playlists = playlistsQuery.data?.items ?? [];
  const viewLabels: Record<MusicView, string> = {
    overview: t("music.views.overview", "Overview"),
    tracks: t("music.views.tracks", "Tracks"),
    albums: t("music.views.albums", "Albums"),
    artists: t("music.views.artists", "Artists"),
    playlists: t("music.views.playlists", "Playlists"),
  };

  const updateParams = (changes: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams);
    next.delete("offset");
    for (const [key, value] of Object.entries(changes)) {
      if (value) next.set(key, value);
      else next.delete(key);
    }
    setSearchParams(next);
  };

  const playAll = () => void playSource(sourceFor(query, likedOnly, sort || undefined));

  const activeQuery =
    view === "tracks"
      ? tracksQuery
      : view === "artists"
        ? artistsQuery
        : view === "playlists"
          ? playlistsQuery
          : albumsQuery;
  const total = activeQuery.data?.total ?? 0;
  const title = likedOnly ? t("music.quickStart.liked", "Liked tracks") : t("music.title");

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title={title} icon={<Music2 className="h-6 w-6 text-primary" />} />
      <div
        ref={scroll}
        className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden px-3 pb-8 sm:px-4"
      >
        <div className="music-browse-content">
          {showHighlights && (
            <section className="music-highlights">
              <div className="music-likes">
                <Link
                  to="/music?view=tracks&liked=1"
                  className="music-likes-fill"
                  aria-label={t("music.quickStart.liked", "Liked tracks")}
                >
                  <MusicLyricExcerpt trackId={highlightQuery.data?.items?.[0]?.track_id} />
                </Link>
                <div className="flex items-end justify-between gap-4">
                  <div>
                    <Link
                      to="/music?view=tracks&liked=1"
                      className="text-2xl font-bold tracking-tight hover:underline"
                    >
                      {t("music.quickStart.liked", "Liked tracks")}
                    </Link>
                    <p className="mt-1 text-sm opacity-70">
                      {likedQuery.data?.total ?? 0} {t("music.tracks.count", "tracks")}
                    </p>
                  </div>
                  <button
                    type="button"
                    className="music-likes-play"
                    disabled={!likedQuery.data?.total}
                    onClick={() => void playSource({ kind: "liked" })}
                    aria-label={t("music.actions.playLiked", "Play liked tracks")}
                  >
                    <Play className="size-5" fill="currentColor" />
                  </button>
                </div>
              </div>
              <div className="min-w-0">
                <div className="music-highlight-tracks">
                  {highlightQuery.isPending && <LoadingState />}
                  {highlightQuery.data?.items?.map((track) => (
                    <MusicTrackRow
                      key={track.track_id}
                      track={track}
                      compact
                      source={{ kind: "liked" }}
                    />
                  ))}
                </div>
              </div>
            </section>
          )}

          <div className="music-browse-main">
            <div className="sticky top-0 z-sticky bg-base-100">
              <div className="music-tabbar" hidden={likedOnly}>
                <div
                  role="tablist"
                  aria-label={t("music.navigation", "Music sections")}
                  className="tabs tabs-box"
                >
                  {views.map(({ id }) => (
                    <Link
                      key={id}
                      to={id === "overview" ? "/music" : `/music?view=${id}`}
                      role="tab"
                      aria-selected={view === id}
                      onClick={() => {
                        stickOnViewChange.current = true;
                      }}
                      className={`tab ${view === id ? "tab-active" : ""}`}
                    >
                      {viewLabels[id]}
                    </Link>
                  ))}
                </div>
              </div>

              <div className="music-toolbar">
                <form
                  role="search"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void navigate(
                      `/music?view=tracks${search.trim() ? `&q=${encodeURIComponent(search.trim())}` : ""}`,
                    );
                  }}
                >
                  <label className="input input-bordered input-sm flex w-48 items-center gap-2">
                    <Search className="size-4 text-base-content/50" />
                    <input
                      type="text"
                      className="grow"
                      placeholder={t("music.search.label", "Search music")}
                      value={search}
                      onChange={(event) => setSearch(event.target.value)}
                      aria-label={t("music.search.label", "Search music")}
                    />
                  </label>
                </form>

                {view === "tracks" && (
                  <>
                    <MusicSortDropdown
                      sort={sort}
                      onSortChange={(value) => updateParams({ sort: value || undefined })}
                    />
                    <MusicFavoriteFilter
                      active={likedOnly}
                      onChange={(value) => updateParams({ liked: value ? "1" : undefined })}
                    />
                    <button
                      type="button"
                      className="btn btn-sm gap-1.5 btn-primary"
                      onClick={playAll}
                      disabled={tracks.length === 0}
                    >
                      <Play className="size-4" />
                      {t("music.actions.playAll", "Play all")}
                    </button>
                  </>
                )}

                {(view === "albums" || view === "artists") && (
                  <MusicFavoriteFilter
                    active={favoritesOnly}
                    onChange={(value) => updateParams({ favorites: value ? "1" : undefined })}
                  />
                )}

                {view === "playlists" && <MusicPlaylistAction variant="primary" />}
              </div>
            </div>

            <MusicPanel active={view === "tracks"}>
              {tracksQuery.isError ? (
                <MusicLoadError onRetry={() => void tracksQuery.refetch()} />
              ) : (
                <TracksView
                  tracks={tracks}
                  total={tracksQuery.data?.total ?? 0}
                  query={query}
                  likedOnly={likedOnly}
                  sort={sort}
                  isLoading={tracksQuery.isPending}
                />
              )}
            </MusicPanel>

            <MusicPanel active={view === "albums"}>
              {albumsQuery.isError ? (
                <MusicLoadError onRetry={() => void albumsQuery.refetch()} />
              ) : (
                <AlbumsView albums={albums} isLoading={albumsQuery.isPending} />
              )}
            </MusicPanel>

            <MusicPanel active={view === "artists"}>
              {artistsQuery.isError ? (
                <MusicLoadError onRetry={() => void artistsQuery.refetch()} />
              ) : (
                <ArtistsView artists={artists} isLoading={artistsQuery.isPending} />
              )}
            </MusicPanel>

            <MusicPanel active={view === "playlists"}>
              {playlistsQuery.isError ? (
                <MusicLoadError onRetry={() => void playlistsQuery.refetch()} />
              ) : (
                <PlaylistsView playlists={playlists} isLoading={playlistsQuery.isPending} />
              )}
            </MusicPanel>
            {view === "overview" && total > albums.length && (
              <Link className="btn btn-ghost" to="/music?view=albums">
                {t("music.albums.viewAll", "View all albums")}
              </Link>
            )}
            {view !== "overview" && total > 50 && (
              <nav
                className="flex items-center justify-between gap-3"
                aria-label={t("music.pagination.label", "Music pages")}
              >
                <button
                  type="button"
                  className="btn btn-sm"
                  disabled={offset === 0 || activeQuery.isFetching}
                  onClick={() => updateParams({ offset: String(Math.max(0, offset - 50)) })}
                >
                  {t("common.previous", "Previous")}
                </button>
                <span className="text-sm text-base-content/60">
                  {offset + 1}–{Math.min(offset + 50, total)} / {total}
                </span>
                <button
                  type="button"
                  className="btn btn-sm"
                  disabled={offset + 50 >= total || activeQuery.isFetching}
                  onClick={() => updateParams({ offset: String(offset + 50) })}
                >
                  {t("common.next", "Next")}
                </button>
              </nav>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function TracksView({
  tracks,
  total,
  query,
  likedOnly,
  sort,
  isLoading,
}: {
  tracks: MusicTrack[];
  total: number;
  query: string;
  likedOnly: boolean;
  sort: "" | "title" | "artist" | "album" | "track";
  isLoading: boolean;
}) {
  const { t } = useI18n();
  if (isLoading) return <LoadingState />;
  return (
    <section className="py-2">
      <div className="mb-3">
        <h2 className="text-lg font-semibold">
          {likedOnly
            ? t("music.quickStart.liked", "Liked tracks")
            : t("music.tracks.heading", "All audio")}
        </h2>
        <p className="text-sm text-base-content/60">
          {total} {t("music.tracks.count", "tracks")}
        </p>
      </div>
      {tracks.length === 0 ? (
        <EmptyState
          title={t("music.tracks.emptyTitle", "No tracks match")}
          description={t(
            "music.tracks.emptyDescription",
            "Try another search or clear the liked filter.",
          )}
        />
      ) : (
        <MusicTrackList tracks={tracks} source={sourceFor(query, likedOnly, sort || undefined)} />
      )}
    </section>
  );
}

function AlbumsView({ albums, isLoading }: { albums: MusicAlbum[]; isLoading: boolean }) {
  const { t } = useI18n();
  if (isLoading) return <LoadingState />;
  if (albums.length === 0)
    return (
      <EmptyState
        title={t("music.albums.emptyTitle", "No music albums yet")}
        description={t(
          "music.albums.emptyDescription",
          "Albums appear when imported tags provide release information.",
        )}
      />
    );
  return (
    <div className="music-cover-grid">
      {albums.map((album) => (
        <MusicCoverCard
          key={album.album_id}
          to={`/music/albums/${album.album_id}`}
          title={album.title || t("music.album.untitled", "Untitled album")}
          source={{ kind: "album", id: album.album_id }}
          subtitle={album.artists?.map((artist, index) => (
            <span key={artist.artist_id ?? index}>
              {index > 0 && " · "}
              <Link to={`/music/artists/${artist.artist_id}`}>{artist.display_name}</Link>
            </span>
          ))}
        >
          <MusicArtwork
            size="cover"
            assetId={album.cover_asset_id}
            alt={album.title || t("music.album.untitled", "Untitled album")}
          />
        </MusicCoverCard>
      ))}
    </div>
  );
}

function ArtistsView({ artists, isLoading }: { artists: MusicArtist[]; isLoading: boolean }) {
  const { t } = useI18n();
  if (isLoading) return <LoadingState />;
  if (artists.length === 0)
    return (
      <EmptyState
        title={t("music.artists.emptyTitle", "No artists yet")}
        description={t(
          "music.artists.emptyDescription",
          "Artist credits will be collected from your local audio metadata.",
        )}
      />
    );
  return (
    <div className="music-cover-grid">
      {artists.map((artist) => (
        <MusicCoverCard
          key={artist.artist_id}
          to={`/music/artists/${artist.artist_id}`}
          title={artist.display_name || t("music.unknownArtist", "Unknown artist")}
          source={{ kind: "query", artist_id: artist.artist_id }}
          round
        >
          <div className="flex aspect-square items-center justify-center rounded-full bg-base-200 text-base-content/40">
            <Mic2 className="size-12" strokeWidth={1.3} />
          </div>
        </MusicCoverCard>
      ))}
    </div>
  );
}

function PlaylistsView({
  playlists,
  isLoading,
}: {
  playlists: MusicPlaylist[];
  isLoading: boolean;
}) {
  const { t } = useI18n();
  if (isLoading) return <LoadingState />;
  return (
    <div className="space-y-4">
      {playlists.length === 0 ? (
        <EmptyState
          title={t("music.playlists.emptyTitle", "No playlists yet")}
          description={t(
            "music.playlists.emptyDescription",
            "Create a playlist to keep a listening order independent from your queue.",
          )}
        />
      ) : (
        <div className="music-cover-grid">
          {playlists.map((playlist) => (
            <MusicCoverCard
              key={playlist.playlist_id}
              to={`/music/playlists/${playlist.playlist_id}`}
              title={playlist.title || t("music.playlists.untitled", "Untitled playlist")}
              source={{ kind: "playlist", id: playlist.playlist_id }}
              subtitle={`${playlist.entry_count ?? 0} ${t("music.tracks.count", "tracks")}`}
            >
              <MusicArtwork
                assetId={playlist.cover_asset_id}
                alt={playlist.title ?? ""}
                size="cover"
              />
            </MusicCoverCard>
          ))}
        </div>
      )}
    </div>
  );
}

function MusicFavoriteFilter({
  active,
  onChange,
}: {
  active: boolean;
  onChange: (active: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="dropdown dropdown-end">
      <div
        tabIndex={0}
        role="button"
        aria-label={t("music.filter.label", "Filter music")}
        className="btn btn-sm btn-soft btn-info gap-2"
      >
        {active ? t("music.filter.favorites", "Favorites") : t("music.filter.all", "All")}
      </div>
      <ul
        tabIndex={0}
        className="dropdown-content menu bg-base-200 rounded-box z-dropdown w-36 p-2 shadow-xl"
      >
        <li>
          <button onClick={() => onChange(false)} className={!active ? "active" : ""}>
            {t("music.filter.all", "All")}
          </button>
        </li>
        <li>
          <button onClick={() => onChange(true)} className={active ? "active" : ""}>
            {t("music.filter.favorites", "Favorites")}
          </button>
        </li>
      </ul>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex min-h-48 items-center justify-center">
      <span className="loading loading-spinner loading-md text-primary" />
    </div>
  );
}

function MusicPanel({ active, children }: { active: boolean; children: ReactNode }) {
  return (
    <div hidden={!active} aria-hidden={!active}>
      {children}
    </div>
  );
}

function MusicLoadError({ onRetry }: { onRetry: () => void }) {
  const { t } = useI18n();
  return (
    <div
      role="alert"
      className="flex items-center justify-between gap-3 rounded-xl bg-error/10 p-4"
    >
      <p>{t("music.loadError", "Music could not be loaded.")}</p>
      <button type="button" className="btn btn-sm" onClick={onRetry}>
        {t("common.retry", "Retry")}
      </button>
    </div>
  );
}
