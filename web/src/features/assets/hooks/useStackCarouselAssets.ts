import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Asset, StackMemberDTO } from "@/lib/assets/types";
import client from "@/lib/http-commons/client";
import { useAssetStackDetails } from "./useAssetStackDetails";

const isAsset = (value: unknown): value is Asset => {
  if (!value || typeof value !== "object") {
    return false;
  }

  return typeof (value as Record<string, unknown>).asset_id === "string";
};

const unwrapAssetResponse = (response: unknown): Asset | undefined => {
  if (isAsset(response)) {
    return response;
  }

  return undefined;
};

export const normalizeStackMembers = (
  members: StackMemberDTO[],
): StackMemberDTO[] =>
  [...members].sort((left, right) => {
    const leftPosition = left.position ?? Number.MAX_SAFE_INTEGER;
    const rightPosition = right.position ?? Number.MAX_SAFE_INTEGER;

    if (leftPosition !== rightPosition) {
      return leftPosition - rightPosition;
    }

    return 0;
  });

export const resolveStackCarouselAssets = async (
  currentAsset: Asset,
  members: StackMemberDTO[],
): Promise<Asset[]> => {
  const sortedMembers = normalizeStackMembers(members);

  const assets = await Promise.allSettled(
    sortedMembers.map(async (member) => {
      const memberAssetId = member.asset_id;
      if (!memberAssetId) return undefined;

      if (currentAsset.asset_id === memberAssetId) {
        return currentAsset;
      }

      const response = await client.GET("/api/v1/assets/{id}", {
        params: {
          path: {
            id: memberAssetId,
          },
        },
      });



      return unwrapAssetResponse(response.data);
    }),
  );

  return assets
    .flatMap((result) =>
      result.status === "fulfilled" && result.value ? [result.value] : [],
    )
    .filter((asset, index, collection) => {
      if (!asset.asset_id) return false;
      return (
        collection.findIndex(
          (candidate) => candidate.asset_id === asset.asset_id,
        ) === index
      );
    });
};

export const useStackCarouselAssets = (asset: Asset, open: boolean) => {
  const stackQuery = useAssetStackDetails(asset.asset_id, open);
  const members = useMemo(
    () => normalizeStackMembers(stackQuery.data?.stack?.members ?? []),
    [stackQuery.data?.stack?.members],
  );
  const memberSignature = useMemo(
    () =>
      members
        .map((member) => `${member.asset_id ?? "missing"}:${member.position ?? "na"}`)
        .join("|"),
    [members],
  );

  const assetsQuery = useQuery({
    queryKey: ["stack-carousel-assets", asset.asset_id, memberSignature],
    enabled: open && members.length > 0 && !stackQuery.isPending && !stackQuery.isError,
    retry: false,
    queryFn: async () => resolveStackCarouselAssets(asset, members),
  });

  const error = useMemo(() => {
    if (stackQuery.isError) {
      return stackQuery.error instanceof Error
        ? stackQuery.error.message
        : "Failed to load stack details";
    }

    if (assetsQuery.isError) {
      return assetsQuery.error instanceof Error
        ? assetsQuery.error.message
        : "Failed to load stack assets";
    }

    if (
      open &&
      !stackQuery.isPending &&
      !assetsQuery.isPending &&
      members.length > 0 &&
      (assetsQuery.data?.length ?? 0) === 0
    ) {
      return "No stack assets available";
    }

    return null;
  }, [
    assetsQuery.data?.length,
    assetsQuery.error,
    assetsQuery.isError,
    assetsQuery.isPending,
    members.length,
    open,
    stackQuery.error,
    stackQuery.isError,
    stackQuery.isPending,
  ]);

  return {
    assets: assetsQuery.data ?? [],
    isLoading:
      open &&
      (stackQuery.isPending ||
        (members.length > 0 && assetsQuery.isPending) ||
        (members.length === 0 && !stackQuery.isError && !stackQuery.data)),
    error,
    memberCount: stackQuery.data?.stack?.member_count ?? members.length,
  };
};
