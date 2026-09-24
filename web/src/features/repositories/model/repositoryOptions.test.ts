import { describe, expect, it } from "vite-plus/test";
import type { RepositoryOption } from "../types";
import {
  getRepositoryEffectiveState,
  isRepositoryUnavailable,
  isUploadLowSpaceBlocked,
  normalizeRepositoryOptions,
} from "./repositoryOptions";

const uploadReady: RepositoryOption = {
  entityType: "repository",
  id: "repo-1",
  rawName: "Photos",
  role: "regular",
  read: { allowed: true, reasons: [] },
  upload: { allowed: true, reasons: [] },
};

describe("normalizeRepositoryOptions", () => {
  it("maps storage targets into the feature model", () => {
    expect(
      normalizeRepositoryOptions({
        targets: [
          {
            id: "primary-by-role",
            name: "Primary",
            role: "primary",
            read: { allowed: true, reasons: [] },
            upload: { allowed: false, reasons: ["offline"] },
          },
          {
            id: "regular",
            name: "Family",
            role: "regular",
            read: { allowed: true, reasons: [] },
            upload: { allowed: true, reasons: [] },
          },
        ],
      }),
    ).toEqual([
      {
        entityType: "repository",
        id: "primary-by-role",
        rawName: "Primary",
        role: "primary",
        read: { allowed: true, reasons: [] },
        upload: { allowed: false, reasons: ["offline"] },
      },
      {
        entityType: "repository",
        id: "regular",
        rawName: "Family",
        role: "regular",
        read: { allowed: true, reasons: [] },
        upload: { allowed: true, reasons: [] },
      },
    ]);
  });

  it("fails closed when upload admission is missing", () => {
    expect(
      normalizeRepositoryOptions({
        targets: [{ id: "no-admission", name: "Mystery" }],
      }),
    ).toEqual([
      {
        entityType: "repository",
        id: "no-admission",
        rawName: "Mystery",
        role: "regular",
        read: { allowed: false, reasons: [] },
        upload: { allowed: false, reasons: [] },
      },
    ]);
  });

  it("returns an empty list when the response has no targets", () => {
    expect(normalizeRepositoryOptions()).toEqual([]);
    expect(normalizeRepositoryOptions({})).toEqual([]);
  });

  it("derives effective state from closed upload admission reasons", () => {
    expect(getRepositoryEffectiveState(uploadReady)).toBe("active");
    expect(
      getRepositoryEffectiveState({
        ...uploadReady,
        upload: { allowed: false, reasons: ["offline"] },
      }),
    ).toBe("offline");
    expect(
      getRepositoryEffectiveState({
        ...uploadReady,
        upload: { allowed: false, reasons: ["paused", "low_space"] },
      }),
    ).toBe("paused");
    expect(
      getRepositoryEffectiveState({
        ...uploadReady,
        upload: { allowed: false, reasons: ["unknown_reason"] },
      }),
    ).toBe("blocked");
  });

  it("rejects upload when admission is denied", () => {
    const [repository] = normalizeRepositoryOptions({
      targets: [
        {
          id: "paused",
          upload: { allowed: false, reasons: ["paused", "low_space"] },
        },
      ],
    });
    expect(repository && isRepositoryUnavailable(repository)).toBe(true);
    expect(repository && isUploadLowSpaceBlocked(repository)).toBe(true);
  });
});
