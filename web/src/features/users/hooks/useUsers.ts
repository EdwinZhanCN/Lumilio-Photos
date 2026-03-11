import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

type UserDTO = Schemas["dto.UserDTO"];
type ManagedUserDTO = Schemas["dto.ManagedUserDTO"];
type ListUsersResponseDTO = Schemas["dto.ListUsersResponseDTO"];

export type UpdateOwnProfilePayload =
  Schemas["dto.UpdateOwnProfileRequestDTO"];
export type AdminUpdateUserPayload =
  Schemas["dto.AdminUpdateUserRequestDTO"];

export function useUsers(limit = 50, offset = 0): UseQueryResult<
  ApiResult<ListUsersResponseDTO>,
  unknown
> & {
  users: ManagedUserDTO[];
  total: number;
} {
  const query = $api.useQuery(
    "get",
    "/api/v1/users",
    {
      params: {
        query: {
          limit,
          offset,
        },
      },
    },
    {
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<ListUsersResponseDTO>, unknown>;

  return {
    ...query,
    users: query.data?.data?.users ?? [],
    total: query.data?.data?.total ?? 0,
  };
}

export function useUpdateMyProfile() {
  const queryClient = useQueryClient();

  return $api.useMutation("patch", "/api/v1/users/me/profile", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/users"],
      });
    },
  });
}

export function useAdminUpdateUser() {
  const queryClient = useQueryClient();

  return $api.useMutation("patch", "/api/v1/users/{id}", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/users"],
      });
    },
  });
}

export type { UserDTO, ManagedUserDTO };
