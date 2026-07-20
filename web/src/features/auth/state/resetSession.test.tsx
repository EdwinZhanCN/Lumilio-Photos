import { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { usePreferencesStore } from "@/lib/preferences/preferences";
import { resetSession } from "./resetSession.ts";

describe("resetSession", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("isolates user A state before user B can authenticate", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(["user-a", "assets"], { assetId: "private-a" });
    let queryAborted = false;
    const inFlightQuery = queryClient
      .fetchQuery({
        queryKey: ["user-a", "in-flight"],
        queryFn: ({ signal }) =>
          new Promise<never>((_resolve, reject) => {
            signal.addEventListener("abort", () => {
              queryAborted = true;
              reject(new DOMException("Aborted", "AbortError"));
            });
          }),
      })
      .catch(() => undefined);
    localStorage.setItem("auth_token", "token-a");
    localStorage.setItem("refresh_token", "refresh-a");
    localStorage.setItem("lumilio.settings.assets_state", "private filters");
    localStorage.setItem("assets_state_v1", "legacy private filters");
    usePreferencesStore.setState({
      workingRepositoryId: "repository-a",
      browseRepositoryId: "repository-a",
    });
    const resetGlobalState = vi.fn();
    const resetFeatureState = vi.fn();

    await resetSession({
      queryClient,
      resetGlobalState,
      resetFeatureState,
    });
    await inFlightQuery;

    expect(queryAborted).toBe(true);
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0);
    expect(queryClient.getMutationCache().getAll()).toHaveLength(0);
    expect(localStorage.getItem("auth_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(localStorage.getItem("lumilio.settings.assets_state")).toBeNull();
    expect(localStorage.getItem("assets_state_v1")).toBeNull();
    expect(usePreferencesStore.getState().workingRepositoryId).toBeUndefined();
    expect(usePreferencesStore.getState().browseRepositoryId).toBeUndefined();
    expect(resetFeatureState).toHaveBeenCalledOnce();
    expect(resetGlobalState).toHaveBeenCalledOnce();
  });
});
