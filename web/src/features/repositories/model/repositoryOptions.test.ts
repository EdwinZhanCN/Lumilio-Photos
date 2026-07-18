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
        isPrimary: true,
      },
      {
        id: "primary-by-flag",
        name: "",
        path: "",
        role: "regular",
        isPrimary: true,
      },
    ]);
  });

  it("returns an empty list when the response has no repositories", () => {
    expect(normalizeRepositoryOptions()).toEqual([]);
    expect(normalizeRepositoryOptions({})).toEqual([]);
  });
});
