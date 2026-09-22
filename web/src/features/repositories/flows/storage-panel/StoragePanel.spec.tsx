import { expect, test } from "vite-plus/test";
import { page, userEvent } from "vitest/browser";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { formatBytes } from "@/lib/utils/formatters";
import type { StorageViewResponse } from "../../api/useStorageView";
import StoragePanel from "../../routes/StoragePanel";
import "@/styles/App.css";

const GIB = 1024 ** 3;

const storageViewFixture = {
  observed_at: "2026-09-19T00:00:00Z",
  capacity_groups: [
    {
      id: "cg-photos",
      grouping_known: true,
      capacity_known: true,
      total_bytes: 2000 * GIB,
      available_bytes: 700 * GIB,
    },
    {
      id: "cg-archive",
      grouping_known: true,
      capacity_known: true,
      total_bytes: 4000 * GIB,
      available_bytes: 1150 * GIB,
    },
    {
      id: "cg-unknown",
      grouping_known: false,
      capacity_known: false,
    },
  ],
  storage_locations: [
    {
      id: "loc-primary",
      name: "Default Storage",
      kind: "default",
      repository_count: 2,
      can_remove: false,
    },
    {
      id: "loc-archive",
      name: "Archive Disk",
      kind: "external",
      repository_count: 2,
      can_remove: false,
    },
    {
      id: "loc-scratch",
      name: "Scratch Volume",
      kind: "external",
      repository_count: 0,
      can_remove: true,
    },
  ],
  repositories: [
    {
      id: "repo-primary",
      name: "Primary Repository",
      role: "primary",
      storage_location_id: "loc-primary",
      capacity_group_id: "cg-photos",
      mount_path: "/Volumes/Photos",
      filesystem: "apfs",
      reachability: "active",
      activity: "idle",
      asset_count: 12_480,
      verification: { status: "completed" },
    },
    {
      id: "repo-family",
      name: "Family Archive",
      role: "regular",
      storage_location_id: "loc-primary",
      capacity_group_id: "cg-photos",
      mount_path: "/Volumes/Photos",
      filesystem: "apfs",
      reachability: "active",
      activity: "idle",
      asset_count: 8830,
      verification: { status: "completed" },
    },
    {
      id: "repo-2019",
      name: "Archive 2019",
      role: "regular",
      storage_location_id: "loc-archive",
      capacity_group_id: "cg-archive",
      mount_path: "/Volumes/Archive",
      filesystem: "ext4",
      reachability: "active",
      activity: "idle",
      asset_count: 24_910,
      verification: { status: "partial" },
    },
    {
      id: "repo-offline",
      name: "Client Projects",
      role: "regular",
      storage_location_id: "loc-archive",
      capacity_group_id: "cg-unknown",
      reachability: "offline",
      activity: "idle",
      asset_count: 3402,
      verification: { status: "completed" },
    },
    {
      id: "repo-detached",
      name: "Detached Archive",
      role: "regular",
      storage_location_id: "",
      reachability: "identity_error",
      activity: "idle",
      asset_count: 640,
      verification: { status: "never" },
    },
  ],
} satisfies StorageViewResponse;

function mockStoragePanelApis() {
  worker.use(
    http.get("*/api/v1/storage/view", () => HttpResponse.json(storageViewFixture)),
    http.get("*/api/v1/storage/native-capability", () => HttpResponse.json({ available: false })),
    http.get("*/api/v1/storage/native-tasks", () => HttpResponse.json([])),
    http.get("*/api/v1/storage/diagnostics", () => HttpResponse.json({ items: [] })),
    http.get("*/api/v1/storage/audit", () =>
      HttpResponse.json({
        events: [
          {
            event_id: "audit-1",
            occurred_at: "2026-09-19T00:00:00Z",
            action: "repository.verify",
            target_type: "repository",
            target_id: "repo-family",
            actor: "admin",
            result: "succeeded",
          },
        ],
      }),
    ),
    http.get("*/api/v1/events/rebuild/status", () => HttpResponse.json({ pending: false })),
  );
}

async function renderPanel(width = 1280) {
  mockStoragePanelApis();
  await page.viewport(width, 1000);
  return renderWithProviders(<StoragePanel />);
}

test("lists Repositories grouped by Storage Location with capacity as a row column", async () => {
  const screen = await renderPanel();

  await expect.element(screen.getByRole("heading", { name: "Default Storage" })).toBeVisible();
  await expect.element(screen.getByRole("heading", { name: "Archive Disk" })).toBeVisible();

  // The storage label names where each figure was measured; two Repositories on
  // one storage repeat it so the column reads as one storage.
  const photosLabels = screen.getByText("Photos", { exact: true }).all();
  expect(photosLabels.length).toBe(2);

  // Capacity is text in a row column, never a page or Location figure, and
  // never a second visual form. The row count guard keeps this from passing on
  // an empty tree.
  expect(document.querySelectorAll("table tbody tr").length).toBeGreaterThan(4);
  expect(document.querySelectorAll(".radial-progress").length).toBe(0);
});

test("expanding a row shows one plain-text fact grid with no second level", async () => {
  const screen = await renderPanel();

  await screen.getByRole("button", { name: "Family Archive", exact: true }).click();

  await expect.element(screen.getByText("/Volumes/Photos", { exact: true })).toBeVisible();
  await expect
    .element(
      screen.getByText(
        t("storagePanel.capacityValue", {
          defaultValue: "{{available}} available of {{total}}",
          available: formatBytes(700 * GIB),
          total: formatBytes(2000 * GIB),
        }),
        { exact: true },
      ),
    )
    .toBeVisible();

  // One level: no nested disclosure inside the opened row, and no gauge.
  const expanded = document.querySelector("table tbody tr td[colspan]");
  expect(expanded?.querySelector("details")).toBeNull();
  expect(document.querySelectorAll(".radial-progress").length).toBe(0);
  const grid = expanded?.querySelector("dl");
  expect(grid?.className).toContain("sm:grid-cols-2");
});

test("an empty Storage Location states that nothing on disk is deleted", async () => {
  const screen = await renderPanel();

  await expect
    .element(
      screen.getByText(
        t("storagePanel.emptyLocationFacts", {
          defaultValue:
            "Registered Storage Location with no Repositories. Detaching removes the registration only; nothing on disk is deleted.",
        }),
        { exact: true },
      ),
    )
    .toBeVisible();
});

test("an unmeasured storage reports an unknown figure instead of a number", async () => {
  const screen = await renderPanel();

  // Wait for the served rows before counting raw DOM nodes: the counts below
  // are synchronous, and the query has not resolved on the first paint.
  await expect.element(screen.getByText("Client Projects", { exact: true })).toBeVisible();

  // Both an unmeasured storage (grouping unproven) and a Repository with no
  // capacity group at all report unknown rather than a fabricated number.
  const unknownCopy = t("storagePanel.capacityUnknownShort", {
    defaultValue: "Capacity is unknown right now.",
  });
  const unknownFigures = [...document.querySelectorAll("td")].filter(
    (cell) => cell.textContent === unknownCopy,
  );
  expect(unknownFigures.length).toBe(2);
});

test("clicking anywhere on a row opens its facts", async () => {
  const screen = await renderPanel();
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  // No expander icon: the row itself is the target.
  expect(document.querySelector("table tbody svg.lucide-chevron-right")).toBeNull();

  const row = screen.getByText("Family Archive", { exact: true }).element().closest("tr");
  expect(row).not.toBeNull();
  (row as HTMLElement).querySelector<HTMLElement>("td")?.click();

  await expect.element(screen.getByText("/Volumes/Photos", { exact: true })).toBeVisible();
});

test("columns share one alignment and both overflow buttons share one right edge", async () => {
  const screen = await renderPanel();
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  // Data columns read from a common leading edge; only the command column
  // trails, which is what lets the two overflow buttons line up.
  const alignment = [...document.querySelectorAll("table thead th")].map((cell) => ({
    label: cell.textContent?.trim(),
    align: getComputedStyle(cell).textAlign,
  }));
  expect(
    alignment.filter(({ label }) => label !== "Actions").every(({ align }) => align === "left"),
  ).toBe(true);
  expect(alignment.find(({ label }) => label === "Actions")?.align).toBe("right");

  const right = (label: string) => {
    const element = document.querySelector<HTMLElement>(`button[aria-label="${label}"]`);
    expect(element, label).not.toBeNull();
    return window.innerWidth - element!.getBoundingClientRect().right;
  };
  expect(
    Math.abs(
      right("Actions for Default Storage Location") - right("Actions for Primary Repository"),
    ),
  ).toBeLessThanOrEqual(1);

  // The section header and the table cells share one horizontal padding, which
  // is what keeps the leading column and both trailing buttons on one edge.
  const header = document.querySelector("section > header");
  const firstHeaderCell = document.querySelector("table thead th");
  const lastHeaderCell = document.querySelector("table thead th:last-child");
  const padding = (element: Element | null, side: "Left" | "Right") =>
    element ? getComputedStyle(element)[`padding${side}` as const] : null;
  expect(padding(header, "Left")).toBe(padding(firstHeaderCell, "Left"));
  expect(padding(header, "Right")).toBe(padding(lastHeaderCell, "Right"));
});

test("a Storage Location that cannot be removed explains why on the entry itself", async () => {
  const screen = await renderPanel();

  await expect
    .element(screen.getByRole("heading", { name: "Default Storage Location" }))
    .toBeVisible();
  await screen
    .getByRole("button", {
      name: t("storagePanel.locationActions", {
        defaultValue: "Actions for {{name}}",
        // The default Location renders under its canonical product term, not
        // the fixture's raw name.
        name: t("productTerms.defaultStorageLocation"),
      }),
      exact: true,
    })
    .click();

  const remove = screen.getByRole("menuitem", {
    name: t("storagePanel.action.removeLocation", { defaultValue: "Remove Storage Location" }),
    exact: true,
  });
  await expect.element(remove).toBeDisabled();

  const hint = t("storagePanel.removeBlocked", {
    defaultValue: "Remove its Repositories first.",
  });
  // The hint is a tooltip on that entry, not a row of its own.
  const entry = remove.element().closest("li");
  expect(entry?.getAttribute("data-tip")).toBe(hint);
  expect(entry?.className).toContain("tooltip");
  expect(
    [...document.querySelectorAll('[role="menu"] li')].some(
      (item) => item.textContent?.trim() === hint,
    ),
  ).toBe(false);

  // Every entry carries an icon, so the rows read alike.
  for (const item of document.querySelectorAll('[role="menuitem"]')) {
    expect(item.querySelector("svg"), item.textContent ?? "").not.toBeNull();
  }
});

test("the page header overflow menu uses the same portaled menu mechanism", async () => {
  const screen = await renderPanel();
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  const trigger = screen.getByRole("button", {
    name: t("storagePanel.moreActions", { defaultValue: "More storage actions" }),
    exact: true,
  });
  await trigger.click();

  const items = [...document.querySelectorAll('[role="menuitem"]')];
  expect(items.length).toBeGreaterThanOrEqual(3);
  for (const item of items) {
    expect(item.querySelector("svg"), item.textContent ?? "").not.toBeNull();
  }
  // Portaled out of the header, like the section and row menus.
  const header = document.querySelector("header");
  expect(header?.querySelector('[role="menu"]')).toBeNull();
  expect(document.querySelector("body > [role='menu']")).not.toBeNull();
});

test("the view tabs are page content, not page-header chrome", async () => {
  const screen = await renderPanel();
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  const tablist = screen.getByRole("tablist").element();
  // No header ancestor: the tabs must not read as part of the page header, and
  // they must live in the scrolling content area with the sections.
  expect(tablist.closest("header")).toBeNull();
  expect(tablist.closest(".overflow-y-auto")).not.toBeNull();
  expect(document.querySelector("header")?.contains(tablist)).toBe(false);
});

test("the History tab lists the lifecycle audit without row expansion", async () => {
  const screen = await renderPanel();

  await screen
    .getByRole("tab", { name: t("storagePanel.view.history", { defaultValue: "History" }) })
    .click();

  await expect.element(screen.getByText("repository.verify", { exact: true })).toBeVisible();
  // The History list is flat: no disclosure and no expandable row.
  expect(document.querySelector("details")).toBeNull();
  expect([...document.querySelectorAll("button")].map((b) => b.textContent?.trim())).not.toContain(
    "Family Archive",
  );
});

test("Repositories without a registered Storage Location are listed separately", async () => {
  const screen = await renderPanel();

  await expect
    .element(
      screen.getByRole("heading", {
        name: t("storagePanel.unlinked.title", {
          defaultValue: "Repositories without a Storage Location",
        }),
      }),
    )
    .toBeVisible();
  await expect.element(screen.getByText("Detached Archive", { exact: true })).toBeVisible();
  // The state badge resolves its copy through the camelCase locale key, not the
  // raw snake_case enum value.
  await expect
    .element(
      screen.getByText(
        t("storagePanel.state.identityError", { defaultValue: "Identity mismatch" }),
        {
          exact: true,
        },
      ),
    )
    .toBeVisible();
});

test("row commands follow the Repository: the primary offers no removal", async () => {
  const screen = await renderPanel();

  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();
  const removeCopy = t("storagePanel.action.remove", { defaultValue: "Remove from Lumilio" });
  const openMenu = (name: string) =>
    screen.getByRole("button", {
      name: t("storagePanel.actionsFor", { defaultValue: "Actions for {{name}}", name }),
      exact: true,
    });

  // The primary Repository cannot be removed, so its menu offers no entry for it.
  await openMenu("Primary Repository").click();
  await expect
    .element(
      screen.getByRole("menuitem", {
        name: t("storagePanel.action.rename", { defaultValue: "Rename Repository" }),
        exact: true,
      }),
    )
    .toBeVisible();
  expect(
    [...document.querySelectorAll('[role="menuitem"]')].map((b) => b.textContent?.trim()),
  ).not.toContain(removeCopy);
  await userEvent.keyboard("{Escape}");

  await openMenu("Family Archive").click();
  await expect
    .element(screen.getByRole("menuitem", { name: removeCopy, exact: true }))
    .toBeVisible();
});

test("verifying from a row menu queues a scan for that Repository", async () => {
  let scannedRepository = "";
  worker.use(
    http.post("*/api/v1/storage/repositories/:id/verifications", ({ params }) => {
      scannedRepository = String(params.id);
      return HttpResponse.json({ operation_id: "op-1", coalesced: false });
    }),
  );
  const screen = await renderPanel();

  await screen
    .getByRole("button", {
      name: t("storagePanel.actionsFor", {
        defaultValue: "Actions for {{name}}",
        name: "Family Archive",
      }),
      exact: true,
    })
    .click();
  await screen
    .getByRole("menuitem", {
      name: t("storagePanel.action.scan", { defaultValue: "Scan now" }),
      exact: true,
    })
    .click();

  await expect.poll(() => scannedRepository).toBe("repo-family");
});
