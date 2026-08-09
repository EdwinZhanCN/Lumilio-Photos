import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import RepositoryCandidateModal from "./RepositoryCandidateModal";

function repositoryResponse() {
  return {
    id: "550e8400-e29b-41d4-a716-446655440000",
    name: "Archive",
    path: "/storage/archive",
    role: "regular",
    reachability: "active",
    activity: "idle",
  };
}

function serveCandidate(
  candidate: Record<string, unknown>,
  endpoint: "open" | "resolve",
  onBody: (body: unknown) => void,
) {
  worker.use(
    http.get("*/api/v1/repository-candidates", () =>
      HttpResponse.json({ candidates: [candidate] }),
    ),
    http.post(`*/api/v1/repository-candidates/${endpoint}`, async ({ request }) => {
      onBody(await request.json());
      return HttpResponse.json(repositoryResponse());
    }),
  );
}

describe("RepositoryCandidateModal storage risk confirmation", () => {
  it("sends an explicit confirmation when opening a risky candidate", async () => {
    let body: unknown;
    serveCandidate(
      {
        directory_name: "archive",
        name: "Archive",
        classification: "existing_repository",
        can_open: true,
        risk_warnings: ["mount_fingerprint_changed"],
      },
      "open",
      (value) => {
        body = value;
      },
    );
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={vi.fn()} />);
    const action = screen.getByRole("button", { name: t("common.open"), exact: true });
    await expect.element(action).toBeDisabled();
    await screen.getByRole("checkbox").click();
    await expect.element(action).toBeEnabled();
    await action.click();
    await vi.waitFor(() =>
      expect(body).toEqual({ directory_name: "archive", risk_confirmation: true }),
    );
  });

  it.each([
    ["update_location", "manage.repositories.hostAction.updateLocation"],
    ["add_separate", "manage.repositories.hostAction.addSeparate"],
  ] as const)("sends an explicit confirmation for %s", async (resolution, actionKey) => {
    let body: unknown;
    serveCandidate(
      {
        directory_name: "archive-moved",
        name: "Archive",
        classification: "identity_error",
        allowed_resolutions: [resolution],
        risk_warnings: ["mount_fingerprint_changed"],
      },
      "resolve",
      (value) => {
        body = value;
      },
    );
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={vi.fn()} />);
    const action = screen.getByRole("button", { name: t(actionKey), exact: true });
    await expect.element(action).toBeDisabled();
    await screen.getByRole("checkbox").click();
    await action.click();
    if (resolution === "add_separate") {
      await screen
        .getByRole("button", {
          name: t("manage.repositories.candidates.addSeparateConfirm"),
          exact: true,
        })
        .click();
    }
    await vi.waitFor(() =>
      expect(body).toEqual({
        directory_name: "archive-moved",
        resolution,
        risk_confirmation: true,
      }),
    );
  });
});
