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

type Review = { token: string; risks: string[]; problem?: string };

/**
 * One candidate, the Server's review of it, and the command endpoint. The
 * review handler records every request body so a test can prove the review
 * names the exact folder and the command carries the review's token.
 */
function serveCandidate(
  candidate: Record<string, unknown>,
  endpoint: "open" | "resolve",
  review: (count: number) => Review,
  command: (body: unknown, count: number) => Response | undefined = () => undefined,
) {
  const reviews: unknown[] = [];
  const commands: unknown[] = [];
  worker.use(
    http.get("*/api/v1/storage/candidates", () => HttpResponse.json({ candidates: [candidate] })),
    http.post("*/api/v1/storage/assessments", async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      reviews.push(body);
      const result = review(reviews.length);
      return HttpResponse.json({
        token: result.token,
        intent: body.intent,
        storage_location_id: "",
        storage_location_kind: "default",
        directory_name: body.directory_name,
        support: result.problem ? "unsupported" : "supported",
        problem: result.problem,
        risks: result.risks,
        can_create: false,
        marker_state: "valid",
      });
    }),
    http.post(`*/api/v1/storage/candidates/${endpoint}`, async ({ request }) => {
      const body = await request.json();
      commands.push(body);
      return command(body, commands.length) ?? HttpResponse.json(repositoryResponse());
    }),
  );
  return { reviews, commands };
}

function problemBody(type: string, extra: Record<string, unknown>, status = 409) {
  return HttpResponse.json(
    {
      type: `https://lumilio.org/problems/${type}`,
      title: type,
      status,
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      ...extra,
    },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

describe("RepositoryCandidateModal shared review", () => {
  it("reviews the exact folder and opens it with the review's token and acknowledged risks", async () => {
    const served = serveCandidate(
      {
        directory_name: "archive",
        name: "Archive",
        classification: "existing_repository",
        can_open: true,
      },
      "open",
      () => ({ token: "v1.open", risks: ["mount_fingerprint_changed"] }),
    );
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={vi.fn()} />);
    await screen.getByRole("button", { name: t("common.open"), exact: true }).click();
    await vi.waitFor(() =>
      expect(served.reviews[0]).toMatchObject({
        intent: "open_repository",
        directory_name: "archive",
      }),
    );
    await expect
      .element(screen.getByText(t("manage.repositories.risks.mountChanged"), { exact: true }))
      .toBeVisible();
    const confirm = screen.getByRole("button", { name: t("common.open"), exact: true });
    await expect.element(confirm).toBeDisabled();
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(confirm).toBeEnabled();
    await confirm.click();
    await vi.waitFor(() =>
      expect(served.commands[0]).toEqual({
        directory_name: "archive",
        assessment_token: "v1.open",
        acknowledged_risks: ["mount_fingerprint_changed"],
      }),
    );
  });

  it.each([
    [
      "update_location",
      "manage.repositories.hostAction.updateLocation",
      "manage.repositories.hostAction.updateLocation",
    ],
    [
      "add_separate",
      "manage.repositories.hostAction.addSeparate",
      "manage.repositories.candidates.addSeparateConfirm",
    ],
  ] as const)("resolves %s through the same review", async (resolution, actionKey, confirmKey) => {
    const served = serveCandidate(
      {
        directory_name: "archive-moved",
        name: "Archive",
        classification: "identity_error",
        allowed_resolutions: [resolution],
      },
      "resolve",
      () => ({ token: "v1.resolve", risks: ["mount_fingerprint_changed"] }),
    );
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={vi.fn()} />);
    await screen.getByRole("button", { name: t(actionKey), exact: true }).click();
    const confirm = screen.getByRole("button", { name: t(confirmKey), exact: true });
    await expect.element(confirm).toBeDisabled();
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(confirm).toBeEnabled();
    await confirm.click();
    await vi.waitFor(() =>
      expect(served.commands[0]).toEqual({
        directory_name: "archive-moved",
        resolution,
        assessment_token: "v1.resolve",
        acknowledged_risks: ["mount_fingerprint_changed"],
      }),
    );
  });

  it("needs no acknowledgement for a folder the review found without risks", async () => {
    const served = serveCandidate(
      {
        directory_name: "plain",
        name: "Plain",
        classification: "existing_repository",
        can_open: true,
      },
      "open",
      () => ({ token: "v1.plain", risks: [] }),
    );
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={vi.fn()} />);
    await screen.getByRole("button", { name: t("common.open"), exact: true }).click();
    const confirm = screen.getByRole("button", { name: t("common.open"), exact: true });
    await expect.element(confirm).toBeEnabled();
    await confirm.click();
    await vi.waitFor(() =>
      expect(served.commands[0]).toEqual({
        directory_name: "plain",
        assessment_token: "v1.plain",
        acknowledged_risks: [],
      }),
    );
  });

  it("shows the typed refusal of an unsupported folder and offers no command", async () => {
    const served = serveCandidate(
      { directory_name: "nas", name: "NAS", classification: "existing_repository", can_open: true },
      "open",
      () => ({ token: "v1.nas", risks: [], problem: "storage/unsupported-filesystem" }),
    );
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={vi.fn()} />);
    await screen.getByRole("button", { name: t("common.open"), exact: true }).click();
    await expect.element(screen.getByRole("alert")).toBeVisible();
    await expect
      .element(screen.getByRole("button", { name: t("common.open"), exact: true }))
      .toBeDisabled();
    expect(served.commands).toHaveLength(0);
  });

  it("re-reviews the folder when the Server reports changed facts", async () => {
    const served = serveCandidate(
      {
        directory_name: "flaky",
        name: "Flaky",
        classification: "existing_repository",
        can_open: true,
      },
      "open",
      (count) => ({
        token: `v1.flaky-${count}`,
        risks:
          count === 1
            ? ["mount_fingerprint_changed"]
            : ["mount_fingerprint_changed", "removable_storage"],
      }),
      (_body, count) =>
        count === 1
          ? problemBody("storage/confirmation-required", {
              risks: ["mount_fingerprint_changed", "removable_storage"],
              assessment_token: "v1.flaky-2",
            })
          : undefined,
    );
    const onClose = vi.fn();
    const screen = await renderWithProviders(<RepositoryCandidateModal isOpen onClose={onClose} />);
    await screen.getByRole("button", { name: t("common.open"), exact: true }).click();
    const confirm = screen.getByRole("button", { name: t("common.open"), exact: true });
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(confirm).toBeEnabled();
    await confirm.click();

    // Refused: the dialog stays, re-reviews, and the previous acknowledgement
    // no longer counts. Acknowledge once the new review is on screen.
    await vi.waitFor(() => expect(served.commands).toHaveLength(1));
    await expect.element(screen.getByRole("status")).toBeVisible();
    await expect
      .element(screen.getByText(t("manage.repositories.risks.removableStorage"), { exact: true }))
      .toBeVisible();
    await expect.element(confirm).toBeDisabled();
    expect(onClose).not.toHaveBeenCalled();
    await screen
      .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
      .click();
    await expect.element(confirm).toBeEnabled();
    await confirm.click();
    await vi.waitFor(() => expect(served.commands).toHaveLength(2));
    expect(served.commands[1]).toEqual({
      directory_name: "flaky",
      assessment_token: "v1.flaky-2",
      acknowledged_risks: ["mount_fingerprint_changed", "removable_storage"],
    });
    await vi.waitFor(() => expect(onClose).toHaveBeenCalledOnce());
  });
});
