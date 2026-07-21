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
            status: "offline",
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
        status: "offline",
        isPrimary: true,
      },
      {
        id: "primary-by-flag",
        name: "",
        path: "",
        role: "regular",
        status: "active",
        isPrimary: true,
      },
    ]);
  });

  it("falls back to active for a missing or unrecognized status, so uploads are not blocked", () => {
    expect(
      normalizeRepositoryOptions({
        repositories: [{ id: "no-status" }, { id: "bogus", status: "wat" as never }],
      }).map((repository) => repository.status),
    ).toEqual(["active", "active"]);
  });

  it("returns an empty list when the response has no repositories", () => {
    expect(normalizeRepositoryOptions()).toEqual([]);
    expect(normalizeRepositoryOptions({})).toEqual([]);
  });
});
