import { useMemo } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album as AlbumDTO } from "@/lib/albums/types";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Album as ImgStackAlbum } from "../components/ImgStackGrid/ImgStackGrid";

const PAGE_SIZE = 60;
export const ALBUMS_QUERY_KEY = ["get", "/api/v1/albums"] as const;

/**
 * Maps a backend album DTO to the ImgStackGrid Album interface
 */
export const mapAlbumToUI = (
  album: AlbumDTO,
  t: (key: string, options?: any) => string,
): ImgStackAlbum => {
  const coverAssetId = album.display_cover_asset_id ?? album.cover_asset_id;

  return {
    id: String(album.album_id),
    name: album.album_name?.trim() || t("collections.untitled"),
    description: album.description ?? "",
    imageCount: album.asset_count ?? 0,
    coverImages: coverAssetId ? [assetUrls.getThumbnailUrl(coverAssetId, "medium")] : undefined,
    createdAt: album.created_at ? new Date(album.created_at) : new Date(),
    updatedAt: album.updated_at ? new Date(album.updated_at) : new Date(),
    albumType: album.album_type,
  };
};

/**
 * Hook for fetching albums with infinite scrolling/pagination
 */
export function useAlbums(t: (key: string, options?: any) => string, repositoryId?: string) {
  const query = $api.useInfiniteQuery(
    "get",
    "/api/v1/albums",
    {
      params: {
        query: {
          limit: PAGE_SIZE,
          repository_id: repositoryId,
        },
      },
    },
    {
      initialPageParam: 0,
      pageParamName: "offset",
      staleTime: 60_000,
      gcTime: 5 * 60_000,
      getNextPageParam: (lastPage, _pages, lastPageParam) => {
        const offset = Number(lastPageParam ?? 0) || 0;
        const loaded = lastPage.albums?.length ?? 0;
        const total = lastPage.total ?? 0;
        return offset + loaded < total ? offset + loaded : undefined;
      },
    },
  );
  const data = useMemo(
    () =>
      query.data
        ? {
            ...query.data,
            pages: query.data.pages.map((page) => ({
              ...page,
              albums: (page.albums ?? []).map((album) => mapAlbumToUI(album, t)),
            })),
          }
        : undefined,
    [query.data, t],
  );
  return { ...query, data };
}

/** Shared small album list for pickers and mention sources. */
export function useAlbumOptions(enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/albums",
    { params: { query: { limit: 100, offset: 0 } } },
    { enabled, staleTime: 60_000, gcTime: 5 * 60_000 },
  );
}
