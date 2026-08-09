import { describe, expect, it } from "vite-plus/test";
import { normalizeRepositoryOptions } from "./repositoryOptions";

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
        id: "primary-by-role",
        name: "Primary",
        path: "/photos/primary",
        role: "primary",
        rootId: "root-1",
        reachability: "offline",
        activity: "scanning",
        isPrimary: true,
      },
      {
        id: "primary-by-flag",
        name: "",
        path: "",
        role: "regular",
        rootId: "",
        reachability: "recovery_required",
        activity: "idle",
        isPrimary: true,
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
});
