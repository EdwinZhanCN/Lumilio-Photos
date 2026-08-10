import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import PrimaryRepositoryGate from "./PrimaryRepositoryGate";

describe("PrimaryRepositoryGate", () => {
  it("keeps ordinary repositories available while the default storage needs recovery", async () => {
    worker.use(
      http.get("*/api/v1/setup/status", () =>
        HttpResponse.json({
          phase: "ready",
          admin_initialized: true,
          primary_repository_initialized: true,
          runtime_state: "degraded",
          runtime_reason: "storage_recovery_required",
          repository_defaults: {
            default_root: "/configured/storage",
            storage_strategy: "date",
            duplicate_handling: "rename",
          },
        }),
      ),
    );

    const screen = await renderWithProviders(
      <PrimaryRepositoryGate>
        <div>ordinary repository content</div>
      </PrimaryRepositoryGate>,
    );

    await expect
      .element(screen.getByText(t("auth.storageRecovery.title"), { exact: true }))
      .toBeVisible();
    await expect.element(screen.getByText("/configured/storage/primary")).toBeVisible();
    await expect.element(screen.getByText("ordinary repository content")).toBeVisible();
  });

  it("submits the immutable storage strategy for primary creation", async () => {
    let requestBody: unknown;
    worker.use(
      http.get("*/api/v1/setup/status", () =>
        HttpResponse.json({
          phase: "admin_created",
          admin_initialized: true,
          primary_repository_initialized: false,
          repository_defaults: {
            default_root: "/storage",
            storage_strategy: "date",
            duplicate_handling: "rename",
            risk_warnings: ["removable_storage"],
          },
        }),
      ),
      http.post("*/api/v1/repositories", async ({ request }) => {
        requestBody = await request.json();
        return HttpResponse.json({
          repository: {
            id: "00000000-0000-0000-0000-000000000002",
            name: "Primary Storage",
            role: "primary",
            storage_strategy: "cas",
          },
          warnings: [],
        });
      }),
    );

    const screen = await renderWithProviders(
      <PrimaryRepositoryGate>
        <div>ready</div>
      </PrimaryRepositoryGate>,
    );
    await screen
      .getByLabelText(t("manage.repositories.storageStrategy.casLabel"), { exact: false })
      .click();
    const submit = screen.getByRole("button", {
      name: t("auth.primaryRepository.submit"),
      exact: true,
    });
    await expect.element(submit).toBeDisabled();
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(submit).toBeEnabled();
    await submit.click();

    await expect
      .poll(() => requestBody)
      .toEqual({
        name: "Primary Storage",
        role: "primary",
        storage_strategy: "cas",
        risk_confirmation: true,
      });
  });
});
