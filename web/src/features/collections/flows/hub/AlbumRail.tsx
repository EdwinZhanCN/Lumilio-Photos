import { Album as AlbumIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import Rail from "../../components/Rail";
import RailCard from "../../components/RailCard";
import type { AlbumViewModel } from "../../model/album";

type AlbumRailProps = {
  albums: AlbumViewModel[];
  loading?: boolean;
  onAlbumClick?: (album: AlbumViewModel) => void;
};

export default function AlbumRail({ albums, loading = false, onAlbumClick }: AlbumRailProps) {
  const { t } = useI18n();

  return (
    <Rail
      loading={loading}
      isEmpty={albums.length === 0}
      empty={
        <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
          {t("collections.emptyAlbums")}
        </div>
      }
    >
      {albums.map((album) => (
        <RailCard
          key={album.id}
          media={{
            kind: "photo",
            src: album.coverImages?.[0],
            fallbackIcon: AlbumIcon,
          }}
          title={album.name}
          subtitle={t("collections.itemsCount", { count: album.imageCount })}
          onClick={() => onAlbumClick?.(album)}
          className="w-48"
        />
      ))}
    </Rail>
  );
}
