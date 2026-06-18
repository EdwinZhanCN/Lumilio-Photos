import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type {
  DetectDuplicatesResponse,
  DuplicateGroup,
  DuplicateStatus,
  DuplicateSummary,
  ListDuplicateGroupsResponse,
  MergeDuplicateGroupRequest,
  MergeDuplicateGroupResponse,
} from "@/lib/duplicates/types";

function isDuplicateQueryKey(queryKey: readonly unknown[]) {
  return (
    queryKey[0] === "get" &&
    (queryKey[1] === "/api/v1/duplicates/summary" ||
      queryKey[1] === "/api/v1/duplicates/groups")
  );
}

function isAssetsQueryKey(queryKey: readonly unknown[]) {
  return (
    queryKey[0] === "get" &&
    typeof queryKey[1] === "string" &&
    queryKey[1].startsWith("/api/v1/assets")
  );
}

/**
 * Loads the Utilities Rail summary card data.
 */
export function useDuplicateSummary(repositoryId?: string) {
  return $api.useQuery(
    "get",
    "/api/v1/duplicates/summary",
    {
      params: {
        query: repositoryId ? { repository_id: repositoryId } : {},
      },
    },
    {
      select: (response) => response as DuplicateSummary,
    },
  );
}

interface UseDuplicateGroupsOptions {
  repositoryId?: string;
  status?: DuplicateStatus;
  limit?: number;
  offset?: number;
}

/**
 * Loads a page of duplicate groups for the Duplicates review page.
 */
export function useDuplicateGroups({
  repositoryId,
  status = "pending",
  limit = 20,
  offset = 0,
}: UseDuplicateGroupsOptions) {
  return $api.useQuery(
    "get",
    "/api/v1/duplicates/groups",
    {
      params: {
        query: {
          repository_id: repositoryId,
          status,
          limit,
          offset,
        },
      },
    },
    {
      select: (response) => response as ListDuplicateGroupsResponse,
    },
  );
}

interface DetectMutationVariables {
  repositoryId: string;
}

/**
 * Triggers a synchronous duplicate detection run for one repository.
 * On success, every duplicate query is invalidated so the UI reflects the new
 * graph immediately.
 */
export function useDetectDuplicates() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/duplicates/detect");

  return useMutation({
    mutationFn: async ({ repositoryId }: DetectMutationVariables) => {
      const result = await mutation.mutateAsync({
        body: { repository_id: repositoryId },
      });
      return result as DetectDuplicatesResponse;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        predicate: (query) => isDuplicateQueryKey(query.queryKey),
      });
    },
  });
}

interface MergeMutationVariables {
  groupId: string;
  body: MergeDuplicateGroupRequest;
}

export function useMergeDuplicateGroup() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation(
    "post",
    "/api/v1/duplicates/groups/{id}/merge",
  );

  return useMutation({
    mutationFn: async ({ groupId, body }: MergeMutationVariables) => {
      const result = await mutation.mutateAsync({
        params: { path: { id: groupId } },
        body,
      });
      return result as MergeDuplicateGroupResponse;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        predicate: (query) => isDuplicateQueryKey(query.queryKey),
      });
      // Re-fetch the asset list grids too because some assets just got
      // soft-deleted under our feet.
      await queryClient.invalidateQueries({
        predicate: (query) => isAssetsQueryKey(query.queryKey),
      });
    },
  });
}

interface DismissMutationVariables {
  groupId: string;
}

export function useDismissDuplicateGroup() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation(
    "post",
    "/api/v1/duplicates/groups/{id}/dismiss",
  );

  return useMutation({
    mutationFn: async ({ groupId }: DismissMutationVariables) => {
      await mutation.mutateAsync({
        params: { path: { id: groupId } },
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        predicate: (query) => isDuplicateQueryKey(query.queryKey),
      });
    },
  });
}

export type DuplicateGroupItem = DuplicateGroup;

/**
 * Convenience wrapper that exposes the same shape used by feature components.
 */
export function useDuplicateGroupList(options: UseDuplicateGroupsOptions): {
  groups: DuplicateGroup[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  refetch: () => Promise<unknown>;
} {
  const query = useDuplicateGroups(options);
  return {
    groups: query.data?.groups ?? [],
    total: query.data?.total ?? 0,
    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  };
}

// Re-export the assertion helper for components that want the raw `useQuery`
// returned object.
export { useQuery };
