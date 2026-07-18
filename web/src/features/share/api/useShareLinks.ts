import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

export type ShareLinkDTO = components["schemas"]["dto.ShareLinkDTO"];
export type CreateShareLinkRequestDTO = components["schemas"]["dto.CreateShareLinkRequestDTO"];
export type CreateShareLinkResponseDTO = components["schemas"]["dto.CreateShareLinkResponseDTO"];
export type UpdateShareLinkRequestDTO = components["schemas"]["dto.UpdateShareLinkRequestDTO"];

const shareLinksQueryKey = ["get", "/api/v1/share-links"];

/** Owner-scoped share link management: list + create + patch/extend + revoke + delete. */
export function useShareLinks() {
  const queryClient = useQueryClient();

  const listQuery = $api.useQuery("get", "/api/v1/share-links", {});

  const invalidate = useCallback(
    async () => queryClient.invalidateQueries({ queryKey: shareLinksQueryKey }),
    [queryClient],
  );

  const createMutation = $api.useMutation("post", "/api/v1/share-links");
  const updateMutation = $api.useMutation("patch", "/api/v1/share-links/{id}");
  const revokeMutation = $api.useMutation("post", "/api/v1/share-links/{id}/revoke");
  const deleteMutation = $api.useMutation("delete", "/api/v1/share-links/{id}");

  const createShareLink = useCallback(
    async (body: CreateShareLinkRequestDTO): Promise<CreateShareLinkResponseDTO> => {
      const result = await createMutation.mutateAsync({ body });
      await invalidate();
      return result;
    },
    [createMutation, invalidate],
  );

  const updateShareLink = useCallback(
    async (shareId: string, body: UpdateShareLinkRequestDTO): Promise<ShareLinkDTO> => {
      const result = await updateMutation.mutateAsync({ params: { path: { id: shareId } }, body });
      await invalidate();
      return result;
    },
    [invalidate, updateMutation],
  );

  const revokeShareLink = useCallback(
    async (shareId: string): Promise<ShareLinkDTO> => {
      const result = await revokeMutation.mutateAsync({ params: { path: { id: shareId } } });
      await invalidate();
      return result;
    },
    [invalidate, revokeMutation],
  );

  const deleteShareLink = useCallback(
    async (shareId: string): Promise<void> => {
      await deleteMutation.mutateAsync({ params: { path: { id: shareId } } });
      await invalidate();
    },
    [deleteMutation, invalidate],
  );

  return {
    links: listQuery.data?.items ?? [],
    isLoading: listQuery.isLoading,
    error: listQuery.error,
    refetch: listQuery.refetch,
    createShareLink,
    isCreating: createMutation.isPending,
    updateShareLink,
    revokeShareLink,
    isRevoking: revokeMutation.isPending,
    deleteShareLink,
    isDeleting: deleteMutation.isPending,
  };
}
