import { describe, expect, it } from "vite-plus/test";
import {
  getStorageEntityDisplayName,
  normalizeRepositoryRootsResponse,
  normalizeStorageDiagnosticsResponse,
} from "./storageEntities";

const translate = (key: string) => key;

describe("storage entity presentation", () => {
  it("uses semantic identity for system-owned names and preserves ordinary names", () => {
    expect(
      getStorageEntityDisplayName(
        {
          entityType: "storage_location",
          kind: "default",
          rawName: "legacy default name",
          path: "/storage",
        },
        translate,
      ),
    ).toBe("productTerms.defaultStorageLocation");
    expect(
      getStorageEntityDisplayName(
        {
          entityType: "repository",
          role: "primary",
          rawName: "legacy primary name",
          path: "/storage/primary",
        },
        translate,
      ),
    ).toBe("productTerms.primaryRepository");
    expect(
      getStorageEntityDisplayName(
        {
          entityType: "repository",
          role: "regular",
          rawName: "Family Archive",
          path: "/storage/family",
        },
        translate,
      ),
    ).toBe("Family Archive");
  });

  it("normalizes every storage API shape into the same discriminated model", () => {
    const roots = normalizeRepositoryRootsResponse({
      roots: [{ id: "root-1", kind: "default", name: "legacy default name", path: "/storage" }],
    });
    const diagnostics = normalizeStorageDiagnosticsResponse({
      items: [
        {
          target_type: "repository",
          target_id: "repo-1",
          role: "primary",
          name: "legacy primary name",
          path: "/storage/primary",
        },
      ],
    });

    expect(roots.roots?.[0]).toMatchObject({
      entityType: "storage_location",
      kind: "default",
      rawName: "legacy default name",
    });
    expect(diagnostics.items?.[0]).toMatchObject({
      entityType: "repository",
      role: "primary",
      rawName: "legacy primary name",
    });
  });
});
