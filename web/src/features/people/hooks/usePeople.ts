import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useWorkingRepository } from "@/features/settings";
import type {
  FaceClusterRebuildResponse,
  ListPeopleResponse,
  PersonDetail,
  PersonSummaryList,
  UpdatePersonRequest,
} from "../people.types";

export type UsePeopleOptions = {
  limit?: number;
  offset?: number;
  repositoryId?: string;
};

export function usePeople(options: UsePeopleOptions = {}): UseQueryResult<
  ListPeopleResponse,
  unknown
> & {
  people: PersonSummaryList;
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
  ) as UseQueryResult<ListPeopleResponse, unknown>;

  return {
    ...query,
    people: query.data?.people ?? [],
    total: query.data?.total ?? 0,
  };
}

export function useRebuildPeopleClusters(repositoryId?: string): {
  rebuildPeople: () => Promise<FaceClusterRebuildResponse>;
  isRebuilding: boolean;
} {
  const { scopedRepositoryId } = useWorkingRepository();
  const queryClient = useQueryClient();
  const scopedId = repositoryId ?? scopedRepositoryId;

  const mutation = $api.useMutation("post", "/api/v1/people/rebuild", {
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
    rebuildPeople: () =>
      mutation.mutateAsync({
        params: {
          query: scopedId ? { repository_id: scopedId } : {},
        },
      }) as Promise<FaceClusterRebuildResponse>,
    isRebuilding: mutation.isPending,
  };
}

export function usePersonDetails(
  personId?: number,
  repositoryId?: string,
): UseQueryResult<PersonDetail, unknown> & {
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
  ) as UseQueryResult<PersonDetail, unknown>;

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
    person: query.data,
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
