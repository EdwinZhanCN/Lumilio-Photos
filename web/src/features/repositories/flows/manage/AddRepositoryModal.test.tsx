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
    http.get("*/api/v1/repository-roots", () =>
      HttpResponse.json({
        roots: [
          {
            id: "6b767057-d6f1-465c-b816-d1229f622a20",
            name: "Local storage",
            path: "/storage",
            kind: "default",
            status: "active",
          },
        ],
      }),
    ),
  );
}

function repositoryResponse(name: string) {
  return {
    repository: {
      id: crypto.randomUUID(),
      name,
      path: `/storage/${name.toLowerCase().replaceAll(" ", "-")}`,
      role: "regular",
      is_primary: false,
      storage_strategy: "date",
      local_settings: { handle_duplicate_filenames: "rename" },
    },
    warnings: [],
  };
}

describe("AddRepositoryModal", () => {
  it("submits explicit local repository policies", async () => {
    serveCloudCredentials();
    let body: unknown;
    worker.use(
      http.post("*/api/v1/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("Local Archive"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Local Archive");
    await screen
      .getByLabelText(t("manage.repositories.storageStrategyLabel"), { exact: true })
      .selectOptions("flat");
    await screen
      .getByLabelText(t("manage.repositories.duplicateHandlingLabel"), { exact: true })
      .selectOptions("uuid");
    await screen
      .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(body).toEqual({
        name: "Local Archive",
        root_id: "6b767057-d6f1-465c-b816-d1229f622a20",
        storage_strategy: "flat",
        duplicate_handling: "uuid",
      });
    });
  });

  it("uses the same repository policies for cloud-backed creation", async () => {
    serveCloudCredentials();
    let body: unknown;
    worker.use(
      http.post("*/api/v1/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("Cloud Archive"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Cloud Archive");
    await screen
      .getByRole("button", { name: t("manage.repositories.sourceCloud"), exact: true })
      .click();
    await screen
      .getByLabelText(t("manage.repositories.cloudCredentialLabel"), { exact: true })
      .selectOptions("550e8400-e29b-41d4-a716-446655440000");
    await screen
      .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(body).toEqual({
        name: "Cloud Archive",
        root_id: "6b767057-d6f1-465c-b816-d1229f622a20",
        cloud_credential_id: "550e8400-e29b-41d4-a716-446655440000",
        storage_strategy: "date",
        duplicate_handling: "rename",
      });
    });
  });

  it("selects an active external root when the default is offline", async () => {
    serveCloudCredentials();
    let body: unknown;
    worker.use(
      http.get("*/api/v1/repository-roots", () =>
        HttpResponse.json({
          roots: [
            {
              id: "6b767057-d6f1-465c-b816-d1229f622a20",
              name: "Local storage",
              path: "/storage",
              kind: "default",
              status: "offline",
            },
            {
              id: "24162749-4136-4c24-96db-b5056d9cdf20",
              name: "External archive",
              path: "/media/archive",
              kind: "external",
              status: "active",
            },
          ],
        }),
      ),
      http.post("*/api/v1/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("External Archive"));
      }),
    );

    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);

    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("External Archive");
    await screen
      .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(body).toMatchObject({ root_id: "24162749-4136-4c24-96db-b5056d9cdf20" });
    });
  });
});
