import { Album as AlbumIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import type { Album } from "./ImgStackGrid/ImgStackGrid";

type AlbumRailProps = {
  albums: Album[];
  loading?: boolean;
  onAlbumClick?: (album: Album) => void;
};

const AlbumRailSkeleton = () => (
  <div className="flex gap-4 overflow-x-auto pb-2">
    {Array.from({ length: 4 }).map((_, index) => (
      <div key={index} className="w-48 shrink-0">
        <div className="aspect-square animate-pulse rounded-[1.75rem] bg-base-300/70" />
        <div className="mt-3 space-y-2 px-1">
          <div className="h-4 w-28 animate-pulse rounded bg-base-300/70" />
          <div className="h-3 w-16 animate-pulse rounded bg-base-300/50" />
        </div>
      </div>
    ))}
  </div>
);

export default function AlbumRail({
  albums,
  loading = false,
  onAlbumClick,
}: AlbumRailProps) {
  const { t } = useI18n();

  if (loading) {
    return <AlbumRailSkeleton />;
  }

  if (albums.length === 0) {
    return (
      <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
        {t("collections.emptyAlbums")}
      </div>
    );
  }

  return (
    <div className="flex gap-4 overflow-x-auto pb-2">
      {albums.map((album) => (
        <button
          key={album.id}
          type="button"
          onClick={() => onAlbumClick?.(album)}
          className="group w-48 shrink-0 text-left"
        >
          <div className="relative aspect-square overflow-hidden rounded-[1.75rem] bg-base-200 shadow-[0_18px_48px_-32px_rgba(15,23,42,0.45)] transition duration-300 group-hover:-translate-y-0.5 group-hover:shadow-[0_24px_56px_-32px_rgba(15,23,42,0.55)]">
            {album.coverImages?.[0] ? (
              <img
                src={album.coverImages[0]}
                alt={album.name}
                className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]"
                loading="lazy"
              />
            ) : (
              <div className="flex h-full items-center justify-center bg-gradient-to-br from-base-200 via-base-300/70 to-base-200">
                <AlbumIcon className="size-12 text-base-content/35" />
              </div>
            )}
          </div>
          <div className="mt-3 space-y-1 px-1">
            <p className="truncate text-base font-semibold">{album.name}</p>
            <p className="text-sm text-base-content/55">
              {t("collections.itemsCount", { count: album.imageCount })}
            </p>
          </div>
        </button>
      ))}
    </div>
  );
}
