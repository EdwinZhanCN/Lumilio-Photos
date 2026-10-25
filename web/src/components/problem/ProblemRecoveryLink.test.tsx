import { describe, expect, it } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { PROBLEM_RECOVERY, type RecoveryEntry } from "@/lib/http-commons/problemRecovery";
import { ProblemRecoveryLink, recoveryEntryFor, resolveRecoveryRoute } from "./ProblemRecoveryLink";

function problem(type: string, extra: Record<string, unknown> = {}) {
  return {
    type: `https://lumilio.org/problems/${type}`,
    status: 409,
    instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
    ...extra,
  };
}

describe("ProblemRecoveryLink", () => {
  it("knows an entry for every Problem type the contract declares, with a route wherever one is followed", () => {
    for (const [type, entry] of Object.entries(PROBLEM_RECOVERY) as [string, RecoveryEntry][]) {
      expect(recoveryEntryFor(type), type).toEqual(entry);
      if (entry.kind === "route" || entry.kind === "sign_in" || entry.kind === "host_action") {
        expect(entry.route, type).toMatch(/^\//);
      } else {
        expect(entry.route, type).toBeUndefined();
      }
      if (entry.kind === "host_action") {
        expect(entry.hostAction, type).toBeTruthy();
      }
    }
    expect(recoveryEntryFor("https://example.com/not-a-lumilio-problem")).toBeUndefined();
  });

  it("fills the Repository segment from the Problem and falls back to the Storage page without one", () => {
    const trashed = recoveryEntryFor("https://lumilio.org/problems/asset/trashed");
    expect(trashed).toBeDefined();
    expect(
      resolveRecoveryRoute(trashed!, { repositoryID: "2f77ed88-8049-4b85-b060-65efb89565de" }),
    ).toBe("/storage/2f77ed88-8049-4b85-b060-65efb89565de/trash");
    expect(resolveRecoveryRoute(trashed!, {})).toBe("/storage");
    const busy = recoveryEntryFor("https://lumilio.org/problems/storage/busy");
    expect(resolveRecoveryRoute(busy!, {})).toBeUndefined();
  });

  it("renders the route of a Problem as a link and nothing for a retry", async () => {
    const screen = await renderWithProviders(
      <div>
        <ProblemRecoveryLink
          problem={problem("repository/conflict", {
            conflict_type: "existing_repository_found",
            repository_id: "abc",
          })}
        />
        <ProblemRecoveryLink problem={problem("storage/busy")} />
        <ProblemRecoveryLink problem={problem("storage/identity-mismatch")} />
      </div>,
    );
    const storage = screen.getByRole("link", {
      name: t("problemRecovery.openStorage"),
      exact: true,
    });
    await expect.element(storage).toHaveAttribute("href", "/storage");
    const desktop = screen.getByRole("link", {
      name: t("problemRecovery.desktopApproval"),
      exact: true,
    });
    await expect.element(desktop).toHaveAttribute("data-recovery-kind", "host_action");
    expect(document.querySelectorAll("a[data-recovery-kind]")).toHaveLength(2);
  });
});
