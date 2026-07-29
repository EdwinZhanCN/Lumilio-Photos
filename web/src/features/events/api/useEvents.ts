import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { EventPatch, EventShareRequest } from "../model/event";

const eventsKey = ["get", "/api/v1/events"] as const;

export function useEvents() {
  return $api.useInfiniteQuery(
    "get",
    "/api/v1/events",
    { params: { query: { limit: 50 } } },
    {
      initialPageParam: "",
      pageParamName: "cursor",
      getNextPageParam: (page) => page.next_cursor || undefined,
      staleTime: 60_000,
    },
  );
}

export function useEvent(eventId?: string) {
  const queryClient = useQueryClient();
  const query = $api.useQuery(
    "get",
    "/api/v1/events/{id}",
    { params: { path: { id: eventId ?? "" } } },
    { enabled: Boolean(eventId), refetchOnWindowFocus: false },
  );
  const patchMutation = $api.useMutation("patch", "/api/v1/events/{id}", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: eventsKey }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/events/{id}"] }),
      ]);
    },
  });
  const shareMutation = $api.useMutation("post", "/api/v1/events/{id}/share");
  const splitMutation = $api.useMutation("post", "/api/v1/events/{id}/split");
  const mergeMutation = $api.useMutation("post", "/api/v1/events/merge");
  const addMutation = $api.useMutation("post", "/api/v1/events/{id}/members");
  const removeMutation = $api.useMutation("delete", "/api/v1/events/{id}/members/{mediaItemId}");
  const invalidate = async () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: eventsKey }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/events/{id}"] }),
      queryClient.invalidateQueries({ queryKey: ["post", "/api/v1/assets/list"] }),
    ]);
  return {
    ...query,
    patch: (body: EventPatch) =>
      patchMutation.mutateAsync({ params: { path: { id: eventId ?? "" } }, body }),
    isPatching: patchMutation.isPending,
    share: (body: EventShareRequest) =>
      shareMutation.mutateAsync({ params: { path: { id: eventId ?? "" } }, body }),
    isSharing: shareMutation.isPending,
    split: async (beforeMediaItemId: string) => {
      const result = await splitMutation.mutateAsync({
        params: { path: { id: eventId ?? "" } },
        body: { before_media_item_id: beforeMediaItemId },
      });
      await invalidate();
      return result;
    },
    merge: async (otherEventId: string) => {
      const result = await mergeMutation.mutateAsync({
        body: {
          event_ids: [eventId ?? "", otherEventId],
          survivor_event_id: eventId ?? "",
        },
      });
      await invalidate();
      return result;
    },
    addAssets: async (assetIds: string[], targetEventId = eventId ?? "") => {
      const result = await addMutation.mutateAsync({
        params: { path: { id: targetEventId } },
        body: { asset_ids: assetIds },
      });
      await invalidate();
      return result;
    },
    remove: async (mediaItemId: string) => {
      const result = await removeMutation.mutateAsync({
        params: { path: { id: eventId ?? "", mediaItemId } },
      });
      await invalidate();
      return result;
    },
    isCorrecting:
      splitMutation.isPending ||
      removeMutation.isPending ||
      mergeMutation.isPending ||
      addMutation.isPending,
  };
}

export function useEventRebuild() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/events/rebuild", {
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: eventsKey }),
  });
  return {
    rebuild: (dryRun = false) => mutation.mutateAsync({ body: { dry_run: dryRun } }),
    isRebuilding: mutation.isPending,
  };
}
