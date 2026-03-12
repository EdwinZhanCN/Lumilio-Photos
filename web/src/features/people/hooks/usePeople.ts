import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useWorkingRepository } from "@/features/settings";
import type {
  ApiResult,
  ListPeopleResponse,
  PersonDetail,
  UpdatePersonRequest,
} from "../people.types";

export type UsePeopleOptions = {
  limit?: number;
  offset?: number;
  repositoryId?: string;
};

export function usePeople(
  options: UsePeopleOptions = {},
): UseQueryResult<ApiResult<ListPeopleResponse>, unknown> & {
  people: ListPeopleResponse["people"];
  total: number;
} {
  const { scopedRepositoryId } = useWorkingRepository();
  const repositoryId = options.repositoryId ?? scopedRepositoryId;
  const limit = options.limit ?? 24;
  const offset = options.offset ?? 0;

  const query = $api.useQuery(
    "get",
    "/api/v1/people",
    {
      params: {
        query: {
          repository_id: repositoryId,
          limit,
          offset,
        },
      },
    },
    {
      staleTime: 5 * 60 * 1000,
      gcTime: 30 * 60 * 1000,
      retry: 1,
    },
  ) as UseQueryResult<ApiResult<ListPeopleResponse>, unknown>;

  return {
    ...query,
    people: query.data?.data?.people ?? [],
    total: query.data?.data?.total ?? 0,
  };
}

export function usePersonDetails(
  personId?: number,
  repositoryId?: string,
): UseQueryResult<ApiResult<PersonDetail>, unknown> & {
  person?: PersonDetail;
  renamePerson: (name: string) => Promise<unknown>;
  isRenaming: boolean;
} {
  const { scopedRepositoryId } = useWorkingRepository();
  const queryClient = useQueryClient();
  const scopedId = repositoryId ?? scopedRepositoryId;

  const query = $api.useQuery(
    "get",
    "/api/v1/people/{id}",
    {
      params: {
        path: {
          id: personId ?? 0,
        },
        query: scopedId ? { repository_id: scopedId } : {},
      },
    },
    {
      enabled: Boolean(personId && personId > 0),
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<PersonDetail>, unknown>;

  const renameMutation = $api.useMutation("patch", "/api/v1/people/{id}", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/people"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/people/{id}"],
        }),
      ]);
    },
  });

  return {
    ...query,
    person: query.data?.data,
    renamePerson: (name: string) =>
      renameMutation.mutateAsync({
        params: {
          path: {
            id: personId ?? 0,
          },
          query: scopedId ? { repository_id: scopedId } : {},
        },
        body: {
          name,
        } satisfies UpdatePersonRequest,
      }),
    isRenaming: renameMutation.isPending,
  };
}
