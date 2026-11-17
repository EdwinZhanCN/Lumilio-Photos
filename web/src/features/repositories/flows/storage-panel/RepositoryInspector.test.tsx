import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import RepositoryInspector from "./RepositoryInspector";
import type { RepositoryRow } from "./storageViewModel";

function row(overrides: Partial<RepositoryRow> = {}): RepositoryRow {
  return {
    id: "2f77ed88-8049-4b85-b060-65efb89565de",
    name: "Family Archive",
    role: "regular",
    state: "available",
    assetCount: 12,
    verification: "verified",
    storageLocationId: "loc-1",
    storage: { kind: "system", mountPath: "/" },
    filesystem: "apfs",
    capacity: null,
    actions: ["verify"],
    unavailableActions: [],
    ...overrides,
  };
}

describe("RepositoryInspector", () => {
  it("explains a withheld action with the Server's blockers and alternatives", async () => {
    worker.use(http.get("*/api/v1/storage/diagnostics", () => HttpResponse.json({ items: [] })));
    const screen = await renderWithProviders(
      <RepositoryInspector
        repository={row({
          unavailableActions: [
            {
              action: "rename",
              blockers: ["primary_protected"],
              alternatives: ["default_configuration"],
            },
            { action: "reconnect", blockers: ["original_available"], alternatives: [] },
          ],
        })}
        displayName="Family Archive"
      />,
    );
    await expect
      .element(
        screen.getByText(
          t("storagePanel.inspector.withheld", { action: t("storagePanel.actionName.rename") }),
          { exact: true },
        ),
      )
      .toBeVisible();
    await expect
      .element(
        screen.getByText(
          `${t("apiErrors.storage.actionUnavailable.primaryProtected")} ${t("apiErrors.storage.alternatives.defaultConfiguration")}`,
          { exact: true },
        ),
      )
      .toBeVisible();
    await expect
      .element(
        screen.getByText(t("apiErrors.storage.actionUnavailable.originalAvailable"), {
          exact: true,
        }),
      )
      .toBeVisible();
  });
});
