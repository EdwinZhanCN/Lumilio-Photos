import { useInfiniteQuery } from "@tanstack/react-query";
import { albumService, Album as AlbumDTO } from "@/services/albumService";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Album as ImgStackAlbum } from "../components/ImgStackGrid/ImgStackGrid";

const PAGE_SIZE = 60;

/**
 * Maps a backend album DTO to the ImgStackGrid Album interface
 */
export const mapAlbumToUI = (
  album: AlbumDTO,
  t: (key: string, options?: any) => string
): ImgStackAlbum => {
  return {
    id: String(album.album_id),
    name:
      album.album_name?.trim() ||
      t("collections.untitled", { defaultValue: "Untitled Album" }),
    description: album.description ?? "",
    imageCount: album.asset_count ?? 0,
    coverImages: album.cover_asset_id
      ? [assetUrls.getThumbnailUrl(album.cover_asset_id, "medium")]
      : undefined,
    createdAt: album.created_at ? new Date(album.created_at) : new Date(),
    updatedAt: album.updated_at ? new Date(album.updated_at) : new Date(),
  };
};

/**
 * Hook for fetching albums with infinite scrolling/pagination
 */
export function useAlbums(t: (key: string, options?: any) => string) {
  return useInfiniteQuery({
    queryKey: ["albums"],
    initialPageParam: 0,
    queryFn: async ({ pageParam = 0 }) => {
      const response = await albumService.listAlbums({
        limit: PAGE_SIZE,
        offset: pageParam,
      });

      const payload = response.data?.data;
      const total = payload?.total ?? 0;

      return {
        albums: (payload?.albums ?? []).map((album) => mapAlbumToUI(album, t)),
        total,
        nextOffset: pageParam + PAGE_SIZE < total ? pageParam + PAGE_SIZE : null,
      };
    },
    getNextPageParam: (lastPage) => lastPage.nextOffset ?? undefined,
  });
}
