import { useQueryClient, type QueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useBrowseScope } from "@/features/settings";
import type {
  FaceClusterRebuildResponse,
  ListPeopleResponse,
  ListPersonFacesResponse,
  PersonCorrectionResponse,
  PersonDetail,
  PersonFaceList,
  PersonSummaryList,
  UpdatePersonRequest,
} from "../people.types";

/**
 * Invalidate every query whose results can change after a people correction:
 * the people list/detail, the per-person face list, and asset list/search
 * queries (person filters and gallery membership can shift).
 */
function invalidatePeopleQueries(queryClient: QueryClient): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/people"] }),
    queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/people/{id}"] }),
    queryClient.invalidateQueries({
      queryKey: ["get", "/api/v1/people/{id}/faces"],
    }),
    queryClient.invalidateQueries({
      queryKey: ["post", "/api/v1/people/{id}/assets/list"],
    }),
    queryClient.invalidateQueries({ queryKey: ["post", "/api/v1/assets/list"] }),
  ]);
}

export type UsePeopleOptions = {
  limit?: number;
  offset?: number;
  repositoryId?: string;
  includeHidden?: boolean;
};

/**
 * People list for grid/rail pages. This is the only people hook that follows
 * the browse scope: repository is a read-only display filter on lists, while
 * a person itself is not repository-scoped (detail, faces, and every mutation
 * take no repository at all).
 */
export function usePeople(options: UsePeopleOptions = {}): UseQueryResult<
  ListPeopleResponse,
  unknown
> & {
  people: PersonSummaryList;
  total: number;
} {
  const { scopedRepositoryId } = useBrowseScope();
  const repositoryId = options.repositoryId ?? scopedRepositoryId;
  const limit = options.limit ?? 24;
  const offset = options.offset ?? 0;
  const includeHidden = options.includeHidden ?? false;

  const query = $api.useQuery(
    "get",
    "/api/v1/people",
    {
      params: {
        query: {
          repository_id: repositoryId,
          include_hidden: includeHidden,
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

/**
 * Maintenance action (Manage page). repositoryId only bounds which faces are
 * re-assigned; omitting it rebuilds across all repositories, which is the
 * intended default since people span repositories.
 */
export function useRebuildPeopleClusters(repositoryId?: string): {
  rebuildPeople: () => Promise<FaceClusterRebuildResponse>;
  isRebuilding: boolean;
} {
  const queryClient = useQueryClient();

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
          query: repositoryId ? { repository_id: repositoryId } : {},
        },
      }) as Promise<FaceClusterRebuildResponse>,
    isRebuilding: mutation.isPending,
  };
}

export function usePersonDetails(personId?: number): UseQueryResult<PersonDetail, unknown> & {
  person?: PersonDetail;
  renamePerson: (name: string) => Promise<unknown>;
  isRenaming: boolean;
} {
  const queryClient = useQueryClient();

  const query = $api.useQuery(
    "get",
    "/api/v1/people/{id}",
    {
      params: {
        path: {
          id: personId ?? 0,
        },
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
        },
        body: {
          name,
        } satisfies UpdatePersonRequest,
      }),
    isRenaming: renameMutation.isPending,
  };
}

/**
 * All faces of one person, unscoped: this feeds the correction panel, which
 * must show every face regardless of which repository it lives in.
 */
export function usePersonFaces(
  personId?: number,
  options: { limit?: number; offset?: number } = {},
): UseQueryResult<ListPersonFacesResponse, unknown> & {
  faces: PersonFaceList;
  total: number;
} {
  const limit = options.limit ?? 60;
  const offset = options.offset ?? 0;

  const query = $api.useQuery(
    "get",
    "/api/v1/people/{id}/faces",
    {
      params: {
        path: { id: personId ?? 0 },
        query: {
          limit,
          offset,
        },
      },
    },
    {
      enabled: Boolean(personId && personId > 0),
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ListPersonFacesResponse, unknown>;

  return {
    ...query,
    faces: query.data?.faces ?? [],
    total: query.data?.total ?? 0,
  };
}

export function useMergePeople(): {
  mergePeople: (
    targetPersonId: number,
    sourcePersonIds: number[],
  ) => Promise<PersonCorrectionResponse>;
  isMerging: boolean;
} {
  const queryClient = useQueryClient();

  const mutation = $api.useMutation("post", "/api/v1/people/{id}/merge", {
    onSuccess: () => invalidatePeopleQueries(queryClient),
  });

  return {
    mergePeople: (targetPersonId, sourcePersonIds) =>
      mutation.mutateAsync({
        params: {
          path: { id: targetPersonId },
        },
        body: { source_person_ids: sourcePersonIds },
      }) as Promise<PersonCorrectionResponse>,
    isMerging: mutation.isPending,
  };
}

export function useMoveFace(): {
  moveFace: (
    personId: number,
    faceId: number,
    targetPersonId: number,
  ) => Promise<PersonCorrectionResponse>;
  isMoving: boolean;
} {
  const queryClient = useQueryClient();

  const mutation = $api.useMutation("post", "/api/v1/people/{id}/faces/{faceId}/move", {
    onSuccess: () => invalidatePeopleQueries(queryClient),
  });

  return {
    moveFace: (personId, faceId, targetPersonId) =>
      mutation.mutateAsync({
        params: {
          path: { id: personId, faceId },
        },
        body: { target_person_id: targetPersonId },
      }) as Promise<PersonCorrectionResponse>,
    isMoving: mutation.isPending,
  };
}

export function useRemoveFaceFromPerson(): {
  removeFace: (personId: number, faceId: number) => Promise<PersonCorrectionResponse>;
  isRemoving: boolean;
} {
  const queryClient = useQueryClient();

  const mutation = $api.useMutation("post", "/api/v1/people/{id}/faces/{faceId}/remove", {
    onSuccess: () => invalidatePeopleQueries(queryClient),
  });

  return {
    removeFace: (personId, faceId) =>
      mutation.mutateAsync({
        params: {
          path: { id: personId, faceId },
        },
      }) as Promise<PersonCorrectionResponse>,
    isRemoving: mutation.isPending,
  };
}

export function useSetPersonCover(): {
  setPersonCover: (personId: number, faceId: number) => Promise<PersonCorrectionResponse>;
  isSettingCover: boolean;
} {
  const queryClient = useQueryClient();

  const mutation = $api.useMutation("put", "/api/v1/people/{id}/cover", {
    onSuccess: () => invalidatePeopleQueries(queryClient),
  });

  return {
    setPersonCover: (personId, faceId) =>
      mutation.mutateAsync({
        params: {
          path: { id: personId },
        },
        body: { face_id: faceId },
      }) as Promise<PersonCorrectionResponse>,
    isSettingCover: mutation.isPending,
  };
}

export function useSetPersonHidden(): {
  setPersonHidden: (personId: number, hidden: boolean) => Promise<PersonCorrectionResponse>;
  isUpdatingHidden: boolean;
} {
  const queryClient = useQueryClient();

  const mutation = $api.useMutation("put", "/api/v1/people/{id}/hidden", {
    onSuccess: () => invalidatePeopleQueries(queryClient),
  });

  return {
    setPersonHidden: (personId, hidden) =>
      mutation.mutateAsync({
        params: {
          path: { id: personId },
        },
        body: { hidden },
      }) as Promise<PersonCorrectionResponse>,
    isUpdatingHidden: mutation.isPending,
  };
}
