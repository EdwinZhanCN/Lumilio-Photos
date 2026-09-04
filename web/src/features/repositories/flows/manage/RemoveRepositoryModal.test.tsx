import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import RemoveRepositoryModal from "./RemoveRepositoryModal";

const repository = {
  entityType: "repository" as const,
  id: "00000000-0000-0000-0000-000000000002",
  rawName: "Family Archive",
  path: "/storage/family",
  role: "regular" as const,
  rootId: "00000000-0000-0000-0000-000000000001",
  reachability: "active" as const,
  activity: "idle" as const,
};

describe("RemoveRepositoryModal", () => {
  it("states disk preservation and requires the exact repository name", async () => {
    let removeCalls = 0;
    worker.use(
      http.get("*/api/v1/repositories/:id/removal-impact", () =>
        HttpResponse.json({
          repository_id: repository.id,
          repository_name: repository.rawName,
          asset_count: 2,
          files_preserved: true,
        }),
      ),
      http.delete("*/api/v1/repositories/:id", () => {
        removeCalls++;
        return HttpResponse.json({});
      }),
    );
    const screen = await renderWithProviders(
      <RemoveRepositoryModal repository={repository} isOpen onClose={vi.fn()} />,
    );

    await expect
      .element(screen.getByText(t("manage.repositories.removeSafetyWarning"), { exact: true }))
      .toBeVisible();
    const action = screen.getByRole("button", {
      name: t("manage.repositories.removeAction"),
      exact: true,
    });
    await expect.element(action).toBeDisabled();
    const confirmation = screen.getByLabelText(
      t("manage.repositories.removeConfirmationLabel", { name: repository.rawName }),
      { exact: true },
    );
    await confirmation.fill("Family archive");
    await expect.element(action).toBeDisabled();
    await confirmation.fill(repository.rawName);
    await expect.element(action).toBeEnabled();
    await action.click();
    await vi.waitFor(() => expect(removeCalls).toBe(1));
  });
});
