import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Aperture, Camera, Database, FolderOpen } from "lucide-react";
import { client } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { ApiResult, Album, ListAlbumsResponse } from "@/lib/albums/types";
import type { MentionEntity, MentionType, MentionTypeOption } from "./types";

type Schemas = components["schemas"];
type IndexingRepositoryListResponseDTO =
  Schemas["dto.IndexingRepositoryListResponseDTO"];
type OptionsResponseDTO = Schemas["dto.OptionsResponseDTO"];

const ALBUMS_PAGE_SIZE = 100;
const RESOURCE_ICON_SIZE = 14;

async function fetchAllAlbums(): Promise<Album[]> {
  const albums: Album[] = [];
  let offset = 0;
  let total = Number.POSITIVE_INFINITY;

  while (offset < total) {
    const { data, error } = await client.GET("/api/v1/albums", {
      params: {
        query: {
          limit: ALBUMS_PAGE_SIZE,
          offset,
        },
      },
    });

    if (error) {
      throw new Error("Failed to fetch albums");
    }

    const payload = (data as ApiResult<ListAlbumsResponse> | undefined)?.data;
    const page = payload?.albums ?? [];
    total = payload?.total ?? page.length;
    albums.push(...page);

    if (page.length < ALBUMS_PAGE_SIZE) {
      break;
    }
    offset += ALBUMS_PAGE_SIZE;
  }

  return albums;
}

async function fetchRepositories() {
  const { data, error } = await client.GET("/api/v1/assets/indexing/repositories");
  if (error) {
    throw new Error("Failed to fetch repositories");
  }

  const payload =
    (data as ApiResult<IndexingRepositoryListResponseDTO> | undefined)?.data;
  return payload?.repositories ?? [];
}

async function fetchFilterOptions() {
  const { data, error } = await client.GET("/api/v1/assets/filter-options");
  if (error) {
    throw new Error("Failed to fetch filter options");
  }

  const payload = (data as ApiResult<OptionsResponseDTO> | undefined)?.data;
  return payload ?? {};
}

const byLabel = (a: MentionEntity, b: MentionEntity) =>
  a.label.localeCompare(b.label, "zh-Hans-CN", {
    sensitivity: "base",
    numeric: true,
  });

export function useMentionCatalog() {
  const albumsQuery = useQuery({
    queryKey: ["lumilio", "mention-catalog", "albums"],
    queryFn: fetchAllAlbums,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });

  const repositoriesQuery = useQuery({
    queryKey: ["lumilio", "mention-catalog", "repositories"],
    queryFn: fetchRepositories,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });

  const filterOptionsQuery = useQuery({
    queryKey: ["lumilio", "mention-catalog", "filter-options"],
    queryFn: fetchFilterOptions,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });

  const mentionTypes = useMemo<MentionTypeOption[]>(
    () => [
      {
        type: "album",
        label: "相册",
        desc: "按相册精确筛选",
        icon: <FolderOpen size={RESOURCE_ICON_SIZE} />,
      },
      {
        type: "repository",
        label: "仓库",
        desc: "按仓库精确筛选",
        icon: <Database size={RESOURCE_ICON_SIZE} />,
      },
      {
        type: "camera",
        label: "相机",
        desc: "按相机型号精确筛选",
        icon: <Camera size={RESOURCE_ICON_SIZE} />,
      },
      {
        type: "lens",
        label: "镜头",
        desc: "按镜头型号精确筛选",
        icon: <Aperture size={RESOURCE_ICON_SIZE} />,
      },
    ],
    [],
  );

  const entitiesByType = useMemo<Record<MentionType, MentionEntity[]>>(() => {
    const albums = (albumsQuery.data ?? [])
      .map<MentionEntity>((album) => ({
        id: String(album.album_id ?? ""),
        label: album.album_name?.trim() || `Album ${album.album_id ?? ""}`,
        type: "album",
        meta: `album:${album.album_id ?? ""}`,
        desc:
          typeof album.asset_count === "number"
            ? `${album.asset_count} items`
            : undefined,
        icon: <FolderOpen size={RESOURCE_ICON_SIZE} />,
      }))
      .filter((album) => album.id)
      .sort(byLabel);

    const repositories = (repositoriesQuery.data ?? [])
      .map<MentionEntity>((repository) => ({
        id: repository.id ?? "",
        label: repository.name?.trim() || repository.path?.trim() || "Repository",
        type: "repository",
        meta: repository.path ?? undefined,
        desc: repository.is_primary ? "Primary repository" : undefined,
        icon: <Database size={RESOURCE_ICON_SIZE} />,
      }))
      .filter((repository) => repository.id)
      .sort(byLabel);

    const cameraModels = (filterOptionsQuery.data?.camera_makes ?? [])
      .map((cameraModel) => cameraModel.trim())
      .filter(Boolean)
      .map<MentionEntity>((cameraModel) => ({
        id: cameraModel,
        label: cameraModel,
        type: "camera",
        meta: `camera_model:${cameraModel}`,
        icon: <Camera size={RESOURCE_ICON_SIZE} />,
      }))
      .sort(byLabel);

    const lensModels = (filterOptionsQuery.data?.lenses ?? [])
      .map((lensModel) => lensModel.trim())
      .filter(Boolean)
      .map<MentionEntity>((lensModel) => ({
        id: lensModel,
        label: lensModel,
        type: "lens",
        meta: `lens_model:${lensModel}`,
        icon: <Aperture size={RESOURCE_ICON_SIZE} />,
      }))
      .sort(byLabel);

    return {
      album: albums,
      repository: repositories,
      camera: cameraModels,
      lens: lensModels,
    };
  }, [albumsQuery.data, repositoriesQuery.data, filterOptionsQuery.data]);

  return {
    mentionTypes,
    entitiesByType,
    isLoading:
      albumsQuery.isLoading ||
      repositoriesQuery.isLoading ||
      filterOptionsQuery.isLoading,
  };
}
