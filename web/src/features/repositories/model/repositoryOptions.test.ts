import { describe, expect, it } from "vite-plus/test";
import { isRepositoryUnavailable, normalizeRepositoryOptions } from "./repositoryOptions";

describe("normalizeRepositoryOptions", () => {
  it("normalizes missing fields and both primary indicators", () => {
    expect(
      normalizeRepositoryOptions({
        repositories: [
          {
            id: "primary-by-role",
            name: "Primary",
            path: "/photos/primary",
            role: "primary",
            root_id: "root-1",
            reachability: "offline",
            activity: "scanning",
            is_primary: false,
          },
          {
            id: "primary-by-flag",
            is_primary: true,
          },
        ],
      }),
    ).toEqual([
      {
        entityType: "repository",
        id: "primary-by-role",
        rawName: "Primary",
        path: "/photos/primary",
        role: "primary",
        rootId: "root-1",
        reachability: "offline",
        activity: "scanning",
        pauseReason: "",
      },
      {
        entityType: "repository",
        id: "primary-by-flag",
        rawName: "",
        path: "",
        role: "primary",
        rootId: "",
        reachability: "recovery_required",
        activity: "idle",
        pauseReason: "",
      },
    ]);
  });

  it("fails closed for a missing or unrecognized reachability", () => {
    expect(
      normalizeRepositoryOptions({
        repositories: [
          { id: "no-status" },
          { id: "bogus", reachability: "wat" as never, activity: "wat" as never },
        ],
      }).map((repository) => [repository.reachability, repository.activity]),
    ).toEqual([
      ["recovery_required", "idle"],
      ["recovery_required", "idle"],
    ]);
  });

  it("returns an empty list when the response has no repositories", () => {
    expect(normalizeRepositoryOptions()).toEqual([]);
    expect(normalizeRepositoryOptions({})).toEqual([]);
  });

  it("rejects a paused Repository as an upload target", () => {
    const [repository] = normalizeRepositoryOptions({
      repositories: [
        {
          id: "paused",
          reachability: "active",
          activity: "paused",
          pause_reason: "low_space",
        },
      ],
    });
    expect(repository?.pauseReason).toBe("low_space");
    expect(repository && isRepositoryUnavailable(repository)).toBe(true);
  });
});
