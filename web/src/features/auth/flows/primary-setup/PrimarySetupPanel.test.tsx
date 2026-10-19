import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import PrimarySetupPanel from "./PrimarySetupPanel";

const REPOSITORY_ID = "2d5e0d4e-5d84-4a9f-9a52-0d4f2d2d2f10";

function primaryTarget(overrides: Record<string, unknown>) {
  return {
    setup_complete: false,
    native_host: false,
    repository_defaults: {
      storage_location: "/data/storage",
      strategy: "date",
      duplicate_handling: "rename",
    },
    default_storage: {
      path: "/data/storage",
      registered: true,
      state: "ready",
      marker_state: "valid",
    },
    ...overrides,
  };
}

function review(state: string, overrides: Record<string, unknown> = {}) {
  return {
    state,
    actions:
      state === "creatable"
        ? ["create", "fresh_elsewhere"]
        : state === "adoptable" || state === "layout_repairable"
          ? ["adopt", "fresh_elsewhere"]
          : ["fresh_elsewhere"],
    assessment: {
      token: "v1.setup",
      intent: "setup_primary",
      storage_location_id: "6b767057-d6f1-465c-b816-d1229f622a20",
      storage_location_kind: "default",
      directory_name: "primary",
      support: "supported",
      risks: [],
      can_create: state === "creatable",
      marker_state: state === "creatable" ? "absent" : "valid",
    },
    ...overrides,
  };
}

describe("PrimarySetupPanel", () => {
  it("adopts the leftover Primary Repository with the reviewed identity, token and risks", async () => {
    let adoptBody: unknown;
    let created = false;
    worker.use(
      http.get("*/api/v1/setup/primary-target", () =>
        HttpResponse.json(
          primaryTarget({
            target: review("adoptable", {
              assessment: { ...review("adoptable").assessment, risks: ["removable_storage"] },
              existing: {
                repository_id: REPOSITORY_ID,
                name: "Old Primary",
                marker_version: "1.0",
              },
              recovery: {
                trash_files: 2,
                trash_bytes: 4096,
                studio_records: 1,
                staging_files: 2,
                unknown_files: 1,
                log_files: 2,
                derived_files: 0,
                receipts: 0,
              },
            }),
          }),
        ),
      ),
      http.post("*/api/v1/setup/primary-repository", () => {
        created = true;
        return HttpResponse.json({});
      }),
      http.post("*/api/v1/setup/primary-repository/adopt", async ({ request }) => {
        adoptBody = await request.json();
        return HttpResponse.json({
          repository: { id: REPOSITORY_ID, name: "Old Primary", role: "primary", is_primary: true },
          acknowledged_risks: ["removable_storage"],
          recovery_receipt_id: "11111111-1111-4111-8111-111111111111",
          replayed: false,
        });
      }),
    );
    let outcome: string | undefined;
    const screen = await renderWithProviders(
      <PrimarySetupPanel variant="gate" onCompleted={(result) => void (outcome = result)} />,
    );

    await expect
      .element(screen.getByText(t("auth.primarySetup.adopt.title"), { exact: true }))
      .toBeVisible();
    await expect.element(screen.getByText("Old Primary", { exact: false })).toBeVisible();
    await expect.element(screen.getByText(REPOSITORY_ID, { exact: true })).toBeVisible();
    await expect
      .element(screen.getByText(t("auth.primarySetup.adopt.disclosure"), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByText(t("auth.primarySetup.freshElsewhere.server"), { exact: true }))
      .toBeVisible();
    // No create form: the folder already holds a Repository.
    expect(
      screen.getByRole("button", { name: t("auth.primaryRepository.submit"), exact: true }).query(),
    ).toBeNull();

    const adopt = screen.getByRole("button", {
      name: t("auth.primarySetup.adopt.submit"),
      exact: true,
    });
    await expect.element(adopt).toBeDisabled();
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(adopt).toBeEnabled();
    await adopt.click();

    await expect
      .poll(() => adoptBody)
      .toEqual({
        expected_repository_id: REPOSITORY_ID,
        assessment_token: "v1.setup",
        acknowledged_risks: ["removable_storage"],
      });
    await expect.poll(() => outcome).toBe("adopted");
    expect(created).toBe(false);
  });

  it("never offers to create over unmarked files and points a Desktop host at its Settings", async () => {
    worker.use(
      http.get("*/api/v1/setup/primary-target", () =>
        HttpResponse.json(
          primaryTarget({ native_host: true, target: review("nonempty_unmarked") }),
        ),
      ),
    );
    const screen = await renderWithProviders(
      <PrimarySetupPanel variant="bootstrap" onCompleted={() => {}} />,
    );
    await expect
      .element(screen.getByText(t("auth.primarySetup.blocked.nonemptyUnmarked"), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByText(t("auth.primarySetup.freshElsewhere.desktop"), { exact: true }))
      .toBeVisible();
    expect(
      screen
        .getByRole("button", { name: t("auth.bootstrap.repository.submit"), exact: true })
        .query(),
    ).toBeNull();
    expect(
      screen
        .getByRole("button", { name: t("auth.primarySetup.adopt.submit"), exact: true })
        .query(),
    ).toBeNull();
  });

  it("explains an unusable Default Storage Location instead of a create form", async () => {
    worker.use(
      http.get("*/api/v1/setup/primary-target", () =>
        HttpResponse.json(
          primaryTarget({
            default_storage: {
              path: "/data/storage",
              registered: false,
              state: "marker_invalid",
              marker_state: "corrupt",
              detail: "parse storage location marker",
            },
          }),
        ),
      ),
    );
    const screen = await renderWithProviders(
      <PrimarySetupPanel variant="gate" onCompleted={() => {}} />,
    );
    await expect
      .element(
        screen.getByText(t("auth.primarySetup.defaultStorage.markerInvalid"), { exact: true }),
      )
      .toBeVisible();
    await expect.element(screen.getByText("/data/storage", { exact: true })).toBeVisible();
    expect(
      screen.getByRole("button", { name: t("auth.primaryRepository.submit"), exact: true }).query(),
    ).toBeNull();
  });

  it("creates the Primary Repository when the target is creatable", async () => {
    let requestBody: unknown;
    worker.use(
      http.get("*/api/v1/setup/primary-target", () =>
        HttpResponse.json(primaryTarget({ target: review("creatable") })),
      ),
      http.post("*/api/v1/setup/primary-repository", async ({ request }) => {
        requestBody = await request.json();
        return HttpResponse.json({
          repository: {
            id: "00000000-0000-0000-0000-000000000002",
            name: "Primary Storage",
            role: "primary",
          },
          acknowledged_risks: [],
        });
      }),
    );
    let outcome: string | undefined;
    const screen = await renderWithProviders(
      <PrimarySetupPanel variant="bootstrap" onCompleted={(result) => void (outcome = result)} />,
    );
    await screen
      .getByLabelText(t("manage.repositories.storageStrategy.casLabel"), { exact: false })
      .click();
    const submit = screen.getByRole("button", {
      name: t("auth.bootstrap.repository.submit"),
      exact: true,
    });
    await expect.element(submit).toBeEnabled();
    await submit.click();
    await expect
      .poll(() => requestBody)
      .toEqual({
        name: "Primary Storage",
        storage_strategy: "cas",
        assessment_token: "v1.setup",
        acknowledged_risks: undefined,
      });
    await expect.poll(() => outcome).toBe("created");
  });
});
