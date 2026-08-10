import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import CloudSourcesModal from "./CloudSourcesModal";

const repositoryId = "00000000-0000-0000-0000-000000000002";
const credentialId = "00000000-0000-0000-0000-000000000003";

describe("CloudSourcesModal", () => {
  it("binds a connected account and remote scope after repository creation", async () => {
    let requestBody: unknown;
    worker.use(
      http.get("*/api/v1/repositories/:id/cloud", () => HttpResponse.json({ sources: [] })),
      http.get("*/api/v1/cloud/credentials", () =>
        HttpResponse.json({
          credentials: [
            {
              id: credentialId,
              provider: "icloud",
              provider_title: "iCloud Photos",
              display_name: "Family account",
              masked_identity: "f***@example.com",
              status: "connected",
            },
          ],
        }),
      ),
      http.post("*/api/v1/repositories/:id/cloud/sources", async ({ request }) => {
        requestBody = await request.json();
        return HttpResponse.json({
          run: {
            id: "00000000-0000-0000-0000-000000000004",
            repository_id: repositoryId,
            credential_id: credentialId,
            provider: "icloud",
            status: "queued",
          },
        });
      }),
    );

    const screen = await renderWithProviders(
      <CloudSourcesModal
        repositoryId={repositoryId}
        repositoryName="Family"
        isOpen
        onClose={vi.fn()}
      />,
    );

    await screen
      .getByLabelText(t("cloud.sources.credential"), { exact: false })
      .selectOptions(credentialId);
    await screen.getByLabelText(t("cloud.sources.album"), { exact: false }).fill("Favorites");
    await screen
      .getByRole("button", { name: t("cloud.sources.connectAndImport"), exact: true })
      .click();

    await vi.waitFor(() => {
      expect(requestBody).toEqual({
        credential_id: credentialId,
        remote_scope: { album: "Favorites" },
      });
    });
  });

  it("cancels and resumes the durable receipt for a connected source", async () => {
    const runId = "00000000-0000-0000-0000-000000000004";
    let runStatus = "running";
    let cancelCalls = 0;
    let resumeCalls = 0;
    worker.use(
      http.get("*/api/v1/repositories/:id/cloud", () =>
        HttpResponse.json({
          sources: [
            {
              credential: {
                id: credentialId,
                provider: "icloud",
                provider_title: "iCloud Photos",
                display_name: "Family account",
                masked_identity: "f***@example.com",
                status: "connected",
              },
              remote_scope: { album: "Favorites" },
              latest_run: {
                id: runId,
                repository_id: repositoryId,
                credential_id: credentialId,
                provider: "icloud",
                status: runStatus,
              },
            },
          ],
        }),
      ),
      http.get("*/api/v1/cloud/credentials", () => HttpResponse.json({ credentials: [] })),
      http.post("*/api/v1/cloud/import-runs/:id/cancel", () => {
        cancelCalls++;
        runStatus = "cancelled";
        return HttpResponse.json({ run: { id: runId, status: "cancelled" } });
      }),
      http.post("*/api/v1/cloud/import-runs/:id/resume", () => {
        resumeCalls++;
        runStatus = "completed";
        return HttpResponse.json({
          run: { id: "00000000-0000-0000-0000-000000000005", status: "queued" },
        });
      }),
    );

    const screen = await renderWithProviders(
      <CloudSourcesModal
        repositoryId={repositoryId}
        repositoryName="Family"
        isOpen
        onClose={vi.fn()}
      />,
    );

    await screen.getByRole("button", { name: t("cloud.sources.cancel"), exact: true }).click();
    await vi.waitFor(() => expect(cancelCalls).toBe(1));
    await screen.getByRole("button", { name: t("cloud.sources.resume"), exact: true }).click();
    await vi.waitFor(() => expect(resumeCalls).toBe(1));
    await expect
      .element(screen.getByRole("button", { name: t("cloud.sources.importNow"), exact: true }))
      .toBeVisible();
  });
});
