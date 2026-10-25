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
    acknowledged_risks: [],
  };
}

type AssessmentRequest = {
  intent: string;
  storage_location_id?: string;
  directory_name?: string;
};

/** The Server's review of the exact target the wizard submits; risks and token per call. */
function serveAssessment(
  review: (request: AssessmentRequest) => { token: string; risks: string[]; problem?: string },
) {
  const requests: AssessmentRequest[] = [];
  worker.use(
    http.post("*/api/v1/storage/assessments", async ({ request }) => {
      const body = (await request.json()) as AssessmentRequest;
      requests.push(body);
      const result = review(body);
      return HttpResponse.json({
        token: result.token,
        intent: body.intent,
        storage_location_id: body.storage_location_id ?? "",
        storage_location_kind: "external",
        directory_name: body.directory_name ?? "",
        support: result.problem ? "unsupported" : "supported",
        problem: result.problem,
        risks: result.risks,
        can_create: !result.problem,
        marker_state: "absent",
      });
    }),
  );
  return requests;
}

function riskFreeAssessment() {
  return serveAssessment(() => ({ token: "v1.clear", risks: [] }));
}

function problemBody(type: string, extra: Record<string, unknown> = {}, status = 409) {
  return HttpResponse.json(
    {
      type: `https://lumilio.org/problems/${type}`,
      status,
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      ...extra,
    },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

describe("AddRepositoryModal", () => {
  it("submits the selected immutable storage strategy", async () => {
    serveCloudCredentials();
    riskFreeAssessment();
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
        assessment_token: "v1.clear",
        acknowledged_risks: [],
      });
    });
  });

  it("prefers the default Storage Location when several are registered", async () => {
    serveCloudCredentials();
    riskFreeAssessment();
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

  it("reviews the actual target in Desktop and binds the acknowledgement to its token", async () => {
    // Desktop: native host available, server candidates disabled. The risks
    // come from the Server's review of this Storage Location and folder.
    let body: unknown;
    worker.use(
      http.get("*/api/v1/storage/view", () =>
        HttpResponse.json({
          storage_locations: [
            {
              id: "6b767057-d6f1-465c-b816-d1229f622a20",
              name: "Local storage",
              kind: "default",
              repository_count: 1,
            },
            {
              id: "24162749-4136-4c24-96db-b5056d9cdf20",
              name: "Backup disk",
              kind: "external",
              repository_count: 0,
            },
          ],
          repositories: [],
          capacity_groups: [],
        }),
      ),
      http.post("*/api/v1/storage/repositories", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(repositoryResponse("Travel Archive"));
      }),
    );
    const requests = serveAssessment(() => ({
      token: "v1.removable",
      risks: ["removable_storage", "non_default_location"],
    }));
    const screen = await renderWithProviders(
      <AddRepositoryModal
        isOpen
        canRequestStorageLocation
        onClose={vi.fn()}
        initialStorageLocationId="24162749-4136-4c24-96db-b5056d9cdf20"
      />,
    );
    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Travel Archive");
    await advanceWizard(screen, 3);

    // The review targets the selected external Location, not default candidates.
    await vi.waitFor(() =>
      expect(requests).toEqual([
        {
          intent: "create_repository",
          storage_location_id: "24162749-4136-4c24-96db-b5056d9cdf20",
          directory_name: "Travel Archive",
        },
      ]),
    );
    await expect
      .element(screen.getByText(t("manage.repositories.risks.removableStorage"), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByText(t("manage.repositories.risks.nonDefaultLocation"), { exact: true }))
      .toBeVisible();

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

    await vi.waitFor(() =>
      expect(body).toMatchObject({
        storage_location_id: "24162749-4136-4c24-96db-b5056d9cdf20",
        assessment_token: "v1.removable",
        acknowledged_risks: ["removable_storage", "non_default_location"],
      }),
    );
  });

  it("invalidates the acknowledgement when the Server reports changed facts", async () => {
    serveCloudCredentials();
    let tokens = 0;
    serveAssessment(() => {
      tokens += 1;
      // The second review reports the facts the refusal named, so the dialog
      // visibly changes when it lands.
      return {
        token: `v1.review-${tokens}`,
        risks: tokens === 1 ? ["removable_storage"] : ["removable_storage", "cloud_sync_directory"],
      };
    });
    const creates: unknown[] = [];
    worker.use(
      http.post("*/api/v1/storage/repositories", async ({ request }) => {
        creates.push(await request.json());
        return creates.length === 1
          ? problemBody("storage/confirmation-required", {
              risks: ["removable_storage", "cloud_sync_directory"],
              assessment_token: "v1.review-2",
            })
          : HttpResponse.json(repositoryResponse("Flaky Disk"));
      }),
    );
    const onClose = vi.fn();
    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={onClose} />);
    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Flaky Disk");
    await advanceWizard(screen, 3);
    const submit = screen.getByRole("button", {
      name: t("manage.repositories.createSubmit"),
      exact: true,
    });
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(submit).toBeEnabled();
    await submit.click();

    // The first command was refused: the dialog stays open, re-reviews the
    // target, and the previous acknowledgement no longer counts.
    await vi.waitFor(() => expect(creates).toHaveLength(1));
    expect(creates[0]).toMatchObject({ assessment_token: "v1.review-1" });
    await expect.element(screen.getByRole("status")).toBeVisible();
    await vi.waitFor(() => expect(tokens).toBe(2));
    // Acknowledge only once the new review is on screen: an acknowledgement
    // given while the re-review is still in flight is discarded when the new
    // token lands.
    await expect
      .element(screen.getByText(t("manage.repositories.risks.cloudSyncDirectory"), { exact: true }))
      .toBeVisible();
    await expect.element(submit).toBeDisabled();
    expect(onClose).not.toHaveBeenCalled();

    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(submit).toBeEnabled();
    await submit.click();
    await vi.waitFor(() => expect(creates).toHaveLength(2));
    expect(creates[1]).toMatchObject({
      assessment_token: "v1.review-2",
      acknowledged_risks: ["removable_storage", "cloud_sync_directory"],
    });
    await vi.waitFor(() => expect(onClose).toHaveBeenCalledOnce());
  });

  it("refuses a target the Server classifies as unsupported, with the reason inline", async () => {
    serveCloudCredentials();
    serveAssessment(() => ({
      token: "v1.network",
      risks: [],
      problem: "storage/unsupported-filesystem",
    }));
    const createRepository = vi.fn();
    worker.use(
      http.post("*/api/v1/storage/repositories", () => {
        createRepository();
        return HttpResponse.json(repositoryResponse("Share"));
      }),
    );
    const screen = await renderWithProviders(<AddRepositoryModal isOpen onClose={vi.fn()} />);
    await screen
      .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
      .fill("Share");
    await advanceWizard(screen, 3);

    await expect
      .element(screen.getByText(t("apiErrors.storage.unsupportedFilesystem"), { exact: true }))
      .toBeVisible();
    await expect
      .element(
        screen.getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true }),
      )
      .toBeDisabled();
    expect(createRepository).not.toHaveBeenCalled();
  });

  it("keeps the display name independent from the stable storage folder", async () => {
    serveCloudCredentials();
    riskFreeAssessment();
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
    riskFreeAssessment();
    worker.use(
      http.post("*/api/v1/storage/repositories", () =>
        problemBody("repository/conflict", {
          conflict_type: "existing_repository_found",
          repository_id: "2f77ed88-8049-4b85-b060-65efb89565de",
          actions: ["open"],
        }),
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
      expect(onRecoveryRequired).toHaveBeenCalledWith({
        conflictType: "existing_repository_found",
        storageLocationId: "6b767057-d6f1-465c-b816-d1229f622a20",
        directoryName: "Existing Archive",
        repositoryId: "2f77ed88-8049-4b85-b060-65efb89565de",
      });
    });
  });
});

type RenderedScreen = Awaited<ReturnType<typeof renderWithProviders>>;

async function advanceWizard(screen: RenderedScreen, steps = 1) {
  for (let index = 0; index < steps; index += 1) {
    await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
  }
}
