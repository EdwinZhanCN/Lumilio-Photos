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
            storage_location: "/configured/storage",
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
            storage_location: "/storage",
            storage_strategy: "date",
            duplicate_handling: "rename",
            assessment: {
              token: "v1.setup",
              intent: "setup_primary",
              storage_location_id: "6b767057-d6f1-465c-b816-d1229f622a20",
              storage_location_kind: "default",
              directory_name: "primary",
              support: "supported",
              risks: ["removable_storage"],
              can_create: true,
              marker_state: "absent",
            },
          },
        }),
      ),
      http.post("*/api/v1/setup/primary-repository", async ({ request }) => {
        requestBody = await request.json();
        return HttpResponse.json({
          repository: {
            id: "00000000-0000-0000-0000-000000000002",
            name: "Primary Storage",
            role: "primary",
            storage_strategy: "cas",
          },
          acknowledged_risks: ["removable_storage"],
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
        storage_strategy: "cas",
        assessment_token: "v1.setup",
        acknowledged_risks: ["removable_storage"],
      });
  });
});
