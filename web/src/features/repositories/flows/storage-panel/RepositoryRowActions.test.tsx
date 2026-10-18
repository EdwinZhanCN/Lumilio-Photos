import { describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import RepositoryRowActions from "./RepositoryRowActions";
import type { RepositoryRow } from "./storageViewModel";

function row(overrides: Partial<RepositoryRow>): RepositoryRow {
  return {
    id: "repo-1",
    name: "Archive",
    role: "regular",
    state: "available",
    assetCount: 1,
    verification: "verified",
    storageLocationId: "loc-1",
    storage: { kind: "system", mountPath: "/" },
    filesystem: "apfs",
    capacity: null,
    actions: ["verify", "rename", "detach"],
    unavailableActions: [],
    ...overrides,
  };
}

async function openMenu(subject: RepositoryRow, nativeHostAvailable = true) {
  const screen = await renderWithProviders(
    <RepositoryRowActions
      row={subject}
      onCommand={vi.fn()}
      nativeHostAvailable={nativeHostAvailable}
      isScanning={false}
    />,
  );
  await screen
    .getByRole("button", {
      name: t("storagePanel.actionsFor", {
        defaultValue: "Actions for {{name}}",
        name: subject.name,
      }),
      exact: true,
    })
    .click();
  return [...document.querySelectorAll('[role="menuitem"]')].map((item) =>
    item.textContent?.trim(),
  );
}

const renameCopy = t("storagePanel.action.rename", { defaultValue: "Rename Repository" });
const removeCopy = t("storagePanel.action.remove", { defaultValue: "Remove from Lumilio" });
const locateCopy = t("storagePanel.action.locate", { defaultValue: "Locate on disk" });

// R10: the menu renders from the Server's role-aware actions and never
// invents a protected or inapplicable command.
describe("RepositoryRowActions", () => {
  it("offers rename and remove only when the Server lists them", async () => {
    const items = await openMenu(row({}));
    expect(items).toContain(renameCopy);
    expect(items).toContain(removeCopy);
    expect(items).not.toContain(locateCopy);
  });

  it("never offers rename, remove, or reconnect for the protected Primary", async () => {
    const items = await openMenu(
      row({
        id: "primary",
        name: "Primary",
        role: "primary",
        actions: ["verify"],
        unavailableActions: [
          {
            action: "rename",
            blockers: ["primary_protected"],
            alternatives: ["default_configuration"],
          },
          {
            action: "detach",
            blockers: ["primary_protected"],
            alternatives: ["default_configuration"],
          },
          {
            action: "reconnect",
            blockers: ["primary_protected"],
            alternatives: ["default_configuration"],
          },
        ],
      }),
    );
    expect(items).not.toContain(renameCopy);
    expect(items).not.toContain(removeCopy);
    expect(items).not.toContain(locateCopy);
  });

  const offline = () =>
    row({
      state: "offline",
      actions: ["detach", "reconnect"],
      unavailableActions: [
        { action: "rename", blockers: ["repository_offline"], alternatives: [] },
      ],
    });

  it("offers the native reconnect entry for a positively unavailable Repository on a Desktop host", async () => {
    const items = await openMenu(offline(), true);
    expect(items).toContain(locateCopy);
    expect(items).toContain(removeCopy);
    expect(items).not.toContain(renameCopy);
  });

  it("offers no reconnect entry without a Desktop host even when the Server allows it", async () => {
    const items = await openMenu(offline(), false);
    expect(items).not.toContain(locateCopy);
    expect(items).toContain(removeCopy);
  });
});
