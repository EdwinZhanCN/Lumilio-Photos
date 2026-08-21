import { $api } from "@/lib/http-commons/queryClient";

/** Lazily reads the stored OCR relation for one active photo. */
export function useAssetOCR(assetId?: string, enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/{id}",
    {
      params: {
        path: { id: assetId ?? "" },
        query: {
          include_thumbnails: false,
          include_tags: false,
          include_albums: false,
          include_species: false,
          include_ocr: true,
          include_faces: false,
        },
      },
    },
    {
      enabled: enabled && Boolean(assetId),
      retry: 1,
      staleTime: 60_000,
    },
  );
}
