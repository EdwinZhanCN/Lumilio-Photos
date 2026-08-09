import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import RepositoryGrid from "./RepositoryGrid";
import RepositoryRow from "./RepositoryRow";

describe("RepositoryGrid", () => {
  it("shows unfinished native host actions without requiring a modal to be open", async () => {
    worker.use(
      http.get("*/api/v1/host-actions/native-capability", () =>
        HttpResponse.json({ available: true }),
      ),
      http.get("*/api/v1/host-actions", () =>
        HttpResponse.json([
          {
            id: "5d381a4d-f9a3-4d39-a20d-c30266fead1f",
            request_id: "resume-after-restart",
            kind: "open_repository",
            actor: "web:user:1",
            purpose: "Open an existing photo repository",
            status: "pending",
            expires_at: "2026-08-09T00:00:00Z",
            created_at: "2026-08-08T00:00:00Z",
            updated_at: "2026-08-08T00:00:00Z",
          },
        ]),
      ),
      http.get("*/api/v1/repository-roots", () => HttpResponse.json({ roots: [] })),
      http.get("*/api/v1/cloud/credentials", () => HttpResponse.json({ credentials: [] })),
    );

    const noop = vi.fn();
    const screen = await renderWithProviders(
      <RepositoryGrid
        repositories={[]}
        repositoryIds={[]}
        isLoading={false}
        isError={false}
        isScanning={false}
        isRebuildingPeople={false}
        scanningIds={new Set()}
        detectingIds={new Set()}
        rebuildingLocationId={null}
        onScanRepository={noop}
        onDetectStacks={noop}
        onDuplicateScan={noop}
        onLocationRebuild={noop}
        onCloudImport={noop}
        onScanAll={noop}
        onRebuildPeople={noop}
      />,
    );

    await expect
      .element(screen.getByText("unfinished storage request", { exact: false }))
      .toBeVisible();
  });
});

describe("RepositoryRow", () => {
  it("renames the repository through the rename modal", async () => {
    let renameBody: unknown;
    worker.use(
      http.post("*/api/v1/assets/list", () =>
        HttpResponse.json({ items: [], total_media_items: 0 }),
      ),
      http.get("*/api/v1/repositories/:id/cloud", () => HttpResponse.json({ sources: [] })),
      http.get("*/api/v1/repositories/:id/scans/latest", () => HttpResponse.json({})),
      http.get("*/api/v1/cloud/credentials", () => HttpResponse.json({ credentials: [] })),
      http.post("*/api/v1/repositories/:id/rename", async ({ request }) => {
        renameBody = await request.json();
        return HttpResponse.json({
          id: "9ae85f87-adc0-44e0-92de-37380b217ce5",
          name: "After",
          path: "/storage/stable-folder",
          role: "regular",
          root_id: "a8bdfcf7-f7cf-47fe-bf2e-cce5ea4236a9",
          reachability: "active",
          activity: "idle",
        });
      }),
    );
    const noop = vi.fn();
    const screen = await renderWithProviders(
      <ul className="list">
        <RepositoryRow
          repository={{
            id: "9ae85f87-adc0-44e0-92de-37380b217ce5",
            name: "Before",
            path: "/storage/stable-folder",
            role: "regular",
            rootId: "a8bdfcf7-f7cf-47fe-bf2e-cce5ea4236a9",
            reachability: "active",
            activity: "idle",
            isPrimary: false,
          }}
          rootStatus="active"
          isScanning={false}
          isDetecting={false}
          isDuplicateScanning={false}
          isRebuildingLocation={false}
          isCloudImporting={false}
          onScan={noop}
          onDetectStacks={noop}
          onDuplicateScan={noop}
          onLocationRebuild={noop}
          onCloudImport={noop}
        />
      </ul>,
    );

    await screen
      .getByRole("button", { name: "Repository actions for Before", exact: true })
      .click();
    await screen
      .getByRole("button", { name: t("manage.repositories.rename"), exact: true })
      .click();
    await screen.getByLabelText(t("manage.repositories.renameLabel")).fill("After");
    await screen
      .getByRole("button", { name: t("manage.repositories.renameSubmit"), exact: true })
      .click();
    await expect.poll(() => renameBody).toEqual({ name: "After" });
  });
});
