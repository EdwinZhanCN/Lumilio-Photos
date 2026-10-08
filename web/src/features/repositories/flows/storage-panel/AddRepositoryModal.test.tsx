import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import AddRepositoryModal from "./AddRepositoryModal";

function serveCloudCredentials() {
  worker.use(
    http.get("*/api/v1/cloud/credentials", () =>
      HttpResponse.json({
        credentials: [
          {
            id: "550e8400-e29b-41d4-a716-446655440000",
            provider: "icloud",
            provider_title: "iCloud Photos",
            display_name: "Family iCloud",
            masked_identity: "f***@example.com",
            status: "connected",
          },
        ],
      }),
    ),
    http.get("*/api/v1/storage/view", () =>
      HttpResponse.json({
        storage_locations: [
          {
            id: "6b767057-d6f1-465c-b816-d1229f622a20",
            name: "Local storage",
            kind: "default",
            repository_count: 0,
          },
        ],
        repositories: [],
        capacity_groups: [],
      }),
    ),
  );
}

function repositoryResponse(name: string) {
  return {
    repository: {
      id: crypto.randomUUID(),
      name,
      path: `/storage/${name}`,
      role: "regular",
      is_primary: false,
      storage_strategy: "date",
      local_settings: { handle_duplicate_filenames: "rename" },
    },
    warnings: [],
  };
}

describe("AddRepositoryModal", () => {
  it("submits the selected immutable storage strategy", async () => {
    serveCloudCredentials();
    let body: unknown;
    worker.use(
      http.post("*/api/v1/storage/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("Local Archive"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Local Archive");
    await advanceWizard(screen, 2);
    await screen
      .getByLabelText(t("manage.repositories.storageStrategy.flatLabel"), {
        exact: false,
      })
      .click();
    await advanceWizard(screen);
    expect(body).toBeUndefined();
    const submit = screen.getByRole("button", {
      name: t("manage.repositories.createSubmit"),
      exact: true,
    });
    await expect.element(submit).toBeEnabled();
    await submit.click();

    await vi.waitFor(() => {
      expect(body).toEqual({
        name: "Local Archive",
        directory_name: "Local Archive",
        storage_location_id: "6b767057-d6f1-465c-b816-d1229f622a20",
        storage_strategy: "flat",
        risk_confirmation: false,
      });
    });
  });

  it("prefers the default Storage Location when several are registered", async () => {
    serveCloudCredentials();
    let body: unknown;
    worker.use(
      http.get("*/api/v1/storage/view", () =>
        HttpResponse.json({
          storage_locations: [
            {
              id: "24162749-4136-4c24-96db-b5056d9cdf20",
              name: "External archive",
              kind: "external",
              repository_count: 1,
            },
            {
              id: "6b767057-d6f1-465c-b816-d1229f622a20",
              name: "Local storage",
              kind: "default",
              repository_count: 0,
            },
          ],
          repositories: [],
          capacity_groups: [],
        }),
      ),
      http.post("*/api/v1/storage/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("External Archive"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("External Archive");
    await advanceWizard(screen, 3);
    await screen
      .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(body).toMatchObject({
        storage_location_id: "6b767057-d6f1-465c-b816-d1229f622a20",
        storage_strategy: "date",
      });
    });
  });

  it("requires explicit confirmation before creating on a risky Storage Location", async () => {
    let body: unknown;
    worker.use(
      http.get("*/api/v1/storage/view", () =>
        HttpResponse.json({
          storage_locations: [
            {
              id: "6b767057-d6f1-465c-b816-d1229f622a20",
              name: "Network archive",
              kind: "external",
              repository_count: 0,
            },
          ],
          repositories: [],
          capacity_groups: [],
        }),
      ),
      http.get("*/api/v1/storage/candidates", () =>
        HttpResponse.json({
          candidates: [
            {
              directory_name: "Network Archive",
              can_create: true,
              risk_warnings: ["network_filesystem"],
            },
          ],
        }),
      ),
      http.post("*/api/v1/storage/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("Network Archive"));
      }),
    );
    const screen = await renderWithProviders(
      <AddRepositoryModal isOpen showServerCandidates onClose={vi.fn()} />,
    );
    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Network Archive");
    await advanceWizard(screen, 3);

    const submit = screen.getByRole("button", {
      name: t("manage.repositories.createSubmit"),
      exact: true,
    });
    await expect.element(submit).toBeDisabled();
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(submit).toBeEnabled();
    await submit.click();

    await vi.waitFor(() => expect(body).toMatchObject({ risk_confirmation: true }));
  });

  it("keeps the display name independent from the stable storage folder", async () => {
    serveCloudCredentials();
    let body: unknown;
    worker.use(
      http.post("*/api/v1/storage/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("Family.Media (2026)!"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Family.Media (2026)!");
    await screen
      .getByLabelText(t("manage.repositories.createDirectoryLabel"), { exact: true })
      .fill("family-media-2026");
    await advanceWizard(screen, 3);
    await screen
      .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(body).toMatchObject({
        name: "Family.Media (2026)!",
        directory_name: "family-media-2026",
      });
    });
  });

  it("rejects unsupported directory-name characters before submitting", async () => {
    serveCloudCredentials();
    const createRepository = vi.fn();
    worker.use(
      http.post("*/api/v1/storage/repositories", () => {
        createRepository();
        return HttpResponse.json(repositoryResponse("Invalid"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Family/Media");

    const next = screen.getByRole("button", {
      name: t("common.next"),
      exact: true,
    });
    await expect.element(next).toBeDisabled();
    expect(createRepository).not.toHaveBeenCalled();
  });

  it("transitions a create conflict into the repository recovery flow", async () => {
    serveCloudCredentials();
    worker.use(
      http.post("*/api/v1/storage/repositories", () =>
        HttpResponse.json(
          {
            code: 409,
            message: "A repository already exists at this path",
            conflict_type: "existing_repository_found",
            repository_id: "2f77ed88-8049-4b85-b060-65efb89565de",
            actions: ["open"],
          },
          { status: 409 },
        ),
      ),
    );
    const onClose = vi.fn();
    const onRecoveryRequired = vi.fn();
    const screen = await renderWithProviders(
      <AddRepositoryModal isOpen onClose={onClose} onRecoveryRequired={onRecoveryRequired} />,
    );

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Existing Archive");
    await advanceWizard(screen, 3);
    await screen
      .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(onClose).toHaveBeenCalledOnce();
      expect(onRecoveryRequired).toHaveBeenCalledWith("existing_repository_found");
    });
  });
});

type RenderedScreen = Awaited<ReturnType<typeof renderWithProviders>>;

async function advanceWizard(screen: RenderedScreen, steps = 1) {
  for (let index = 0; index < steps; index += 1) {
    await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
  }
}
