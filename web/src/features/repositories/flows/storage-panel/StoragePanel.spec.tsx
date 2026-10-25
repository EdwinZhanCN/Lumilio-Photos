import { expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { formatBytes } from "@/lib/utils/formatters";
import type { components } from "@/lib/http-commons/schema";
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
      actions: [],
      unavailable_actions: [
        {
          action: "reconnect",
          blockers: ["default_protected"],
          alternatives: ["default_configuration"],
        },
        {
          action: "detach",
          blockers: ["default_protected"],
          alternatives: ["default_configuration"],
        },
      ],
    },
    {
      id: "loc-archive",
      name: "Archive Disk",
      kind: "external",
      repository_count: 2,
      can_remove: false,
      actions: [],
      unavailable_actions: [
        { action: "reconnect", blockers: ["original_available"], alternatives: [] },
        { action: "detach", blockers: ["registered_repositories"], alternatives: [] },
      ],
    },
    {
      id: "loc-scratch",
      name: "Scratch Volume",
      kind: "external",
      repository_count: 0,
      can_remove: true,
      actions: ["detach"],
      unavailable_actions: [
        { action: "reconnect", blockers: ["original_available"], alternatives: [] },
      ],
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
      actions: ["verify"],
      unavailable_actions: [
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
          blockers: ["primary_protected", "original_available"],
          alternatives: ["default_configuration"],
        },
      ],
    },
    {
      id: "repo-family",
      name: "Family Archive",
      role: "regular",
      storage_location_id: "loc-primary",
      capacity_group_id: "cg-photos",
      trash_held_count: 2,
      recovery_item_count: 3,
      mount_path: "/Volumes/Photos",
      filesystem: "apfs",
      reachability: "active",
      activity: "idle",
      asset_count: 8830,
      verification: { status: "completed" },
      actions: ["verify", "rename", "detach"],
      unavailable_actions: [{ action: "reconnect", blockers: ["original_available"] }],
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
      verification: { status: "offline" },
      actions: ["verify", "rename", "detach"],
      unavailable_actions: [{ action: "reconnect", blockers: ["original_available"] }],
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
      actions: ["detach", "reconnect"],
      unavailable_actions: [
        { action: "verify", blockers: ["repository_offline"] },
        { action: "rename", blockers: ["repository_offline"] },
      ],
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
      actions: ["detach", "reconnect"],
      unavailable_actions: [
        { action: "verify", blockers: ["identity_problem"] },
        { action: "rename", blockers: ["identity_problem"] },
      ],
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

  await expect
    .element(screen.getByRole("heading", { name: "Default Storage", exact: false }))
    .toBeVisible();
  await expect
    .element(screen.getByRole("heading", { name: "Archive Disk", exact: false }))
    .toBeVisible();

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

  // The Primary is protected by the Server's role-aware policy: its menu
  // offers neither Rename nor Remove, only what the Server lists.
  const renameCopy = t("storagePanel.action.rename", { defaultValue: "Rename Repository" });
  await openMenu("Primary Repository").click();
  await expect
    .element(
      screen.getByRole("menuitem", {
        name: t("storagePanel.action.scan", { defaultValue: "Scan now" }),
        exact: true,
      }),
    )
    .toBeVisible();
  const primaryItems = [...document.querySelectorAll('[role="menuitem"]')].map((b) =>
    b.textContent?.trim(),
  );
  expect(primaryItems).not.toContain(removeCopy);
  expect(primaryItems).not.toContain(renameCopy);
  await userEvent.keyboard("{Escape}");

  await openMenu("Family Archive").click();
  await expect
    .element(screen.getByRole("menuitem", { name: renameCopy, exact: true }))
    .toBeVisible();
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

// R1 of docs/exec-plans/active/storage-repository-lifecycle.md: in Desktop
// (native host available, no server candidates) a create on a removable
// non-default Storage Location reaches the risk review, is refused until the
// risks are acknowledged against the review's token, and then registers the
// selected child. Rendered through the real StoragePanel composition.
test("Desktop create on a removable external Storage Location reviews, acknowledges, and creates", async () => {
  mockStoragePanelApis();
  const assessments: unknown[] = [];
  let created: unknown;
  worker.use(
    http.get("*/api/v1/storage/native-capability", () => HttpResponse.json({ available: true })),
    http.post("*/api/v1/storage/assessments", async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      assessments.push(body);
      return HttpResponse.json({
        token: "v1.archive-disk",
        intent: "create_repository",
        storage_location_id: body.storage_location_id,
        storage_location_kind: "external",
        directory_name: body.directory_name,
        support: "supported",
        risks: ["removable_storage", "non_default_location"],
        can_create: true,
        marker_state: "absent",
      });
    }),
    http.post("*/api/v1/storage/repositories", async ({ request }) => {
      created = await request.json();
      return HttpResponse.json({
        repository: {
          id: "repo-travel",
          name: "Travel 2026",
          path: "/Volumes/Archive/Travel 2026",
          role: "regular",
          is_primary: false,
          storage_strategy: "date",
          local_settings: { handle_duplicate_filenames: "rename" },
        },
        acknowledged_risks: ["removable_storage", "non_default_location"],
      });
    }),
  );
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);

  await screen
    .getByRole("button", { name: t("storagePanel.action.addRepository"), exact: true })
    .click();
  await screen
    .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
    .fill("Travel 2026");
  await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
  // Choose the removable external Location instead of the preselected default.
  await screen
    .getByLabelText(t("manage.repositories.storageLocationLabel"), { exact: true })
    .selectOptions("loc-archive");
  await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
  await screen.getByRole("button", { name: t("common.next"), exact: true }).click();

  await expect
    .poll(() => assessments)
    .toEqual([
      {
        intent: "create_repository",
        storage_location_id: "loc-archive",
        directory_name: "Travel 2026",
      },
    ]);
  await expect
    .element(screen.getByText(t("manage.repositories.risks.removableStorage"), { exact: true }))
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

  await expect
    .poll(() => created)
    .toEqual({
      name: "Travel 2026",
      directory_name: "Travel 2026",
      storage_location_id: "loc-archive",
      storage_strategy: "date",
      assessment_token: "v1.archive-disk",
      acknowledged_risks: ["removable_storage", "non_default_location"],
    });
});

// The create handoff of R9: a create that finds an existing Repository hands
// the user to Open Existing with the folder retained, instead of a toast.
test("a create conflict continues in Open Existing with the folder highlighted", async () => {
  mockStoragePanelApis();
  worker.use(
    http.post("*/api/v1/storage/assessments", async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      return HttpResponse.json({
        token: "v1.default",
        intent: "create_repository",
        storage_location_id: body.storage_location_id,
        storage_location_kind: "default",
        directory_name: body.directory_name,
        support: "supported",
        risks: [],
        can_create: true,
        marker_state: "absent",
      });
    }),
    http.post("*/api/v1/storage/repositories", () =>
      HttpResponse.json(
        {
          type: "https://lumilio.org/problems/repository/conflict",
          status: 409,
          instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
          conflict_type: "existing_repository_found",
          repository_id: "2f77ed88-8049-4b85-b060-65efb89565de",
          actions: ["open"],
        },
        { status: 409, headers: { "Content-Type": "application/problem+json" } },
      ),
    ),
    http.get("*/api/v1/storage/candidates", () =>
      HttpResponse.json({
        candidates: [
          {
            directory_name: "Old Archive",
            name: "Old Archive",
            classification: "existing_repository",
            can_open: true,
            can_create: false,
          },
        ],
      }),
    ),
  );
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);

  await screen
    .getByRole("button", { name: t("storagePanel.action.addRepository"), exact: true })
    .click();
  await screen
    .getByLabelText(t("manage.repositories.createNameLabel"), { exact: true })
    .fill("Old Archive");
  for (let step = 0; step < 3; step += 1) {
    await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
  }
  const submit = screen.getByRole("button", {
    name: t("manage.repositories.createSubmit"),
    exact: true,
  });
  await expect.element(submit).toBeEnabled();
  await submit.click();

  await expect
    .element(screen.getByRole("heading", { name: t("manage.repositories.candidates.title") }))
    .toBeVisible();
  await expect
    .poll(() => document.querySelector("[data-focused='true']")?.textContent)
    .toContain("Old Archive");
});

type HostActionTask = Record<string, unknown>;

/**
 * The native host-action harness: the Desktop and Server replaced by one
 * mutable task whose status the GET poll reports, with every create and
 * resolve request recorded. `kind` and `status` drive the modal exactly as
 * the durable task does in production.
 */
function mockNativeTask(initial: HostActionTask) {
  let task: HostActionTask = { ...initial };
  const created: { idempotencyKey: string | null; body: HostActionTask }[] = [];
  const resolved: HostActionTask[] = [];
  const now = "2026-10-18T00:00:00Z";
  const base = {
    id: "task-1",
    request_id: "request-1",
    actor: "web:user:1",
    expires_at: "2026-10-18T00:10:00Z",
    created_at: now,
    updated_at: now,
  };
  worker.use(
    http.get("*/api/v1/storage/native-capability", () => HttpResponse.json({ available: true })),
    http.post("*/api/v1/storage/native-tasks", async ({ request }) => {
      const body = (await request.json()) as HostActionTask;
      created.push({ idempotencyKey: request.headers.get("Idempotency-Key"), body });
      return HttpResponse.json({ ...base, kind: body.kind, status: "pending" });
    }),
    http.get("*/api/v1/storage/native-tasks/:id", () => HttpResponse.json({ ...base, ...task })),
    http.post("*/api/v1/storage/native-tasks/:id/resolve", async ({ request }) => {
      resolved.push((await request.json()) as HostActionTask);
      task = { ...task, status: "succeeded", result: { name: "Archive Disk" } };
      return HttpResponse.json({ ...base, ...task });
    }),
  );
  return {
    created,
    resolved,
    set(next: HostActionTask) {
      task = { ...task, ...next };
    },
  };
}

async function sendAuthorizeRequest(screen: Awaited<ReturnType<typeof renderPanel>>, name: string) {
  await screen
    .getByRole("button", {
      name: t("storagePanel.moreActions", { defaultValue: "More storage actions" }),
      exact: true,
    })
    .click();
  await screen.getByRole("menuitem", { name: t("storagePanel.authorize"), exact: true }).click();
  await screen
    .getByLabelText(t("manage.repositories.hostAction.locationName"), { exact: true })
    .fill(name);
  await screen
    .getByRole("button", { name: t("manage.repositories.hostAction.send"), exact: true })
    .click();
}

// R9 task outcomes (P3b): a folder Desktop chose on a removable disk stops at
// the shared risk review in the Web. The review renders the Server's risk
// codes, the confirm is disabled until they are acknowledged, and the
// acknowledgement is sent by code, like the create review.
test("a native folder on a removable disk is reviewed and acknowledged by risk code", async () => {
  mockStoragePanelApis();
  const native = mockNativeTask({
    kind: "authorize_storage_location",
    status: "needs_decision",
    result: {
      conflict: {
        type: "storage_risk",
        allowed_resolutions: ["confirm_risk"],
        risk_warnings: ["removable_storage", "non_default_location"],
      },
    },
  });
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  await sendAuthorizeRequest(screen, "Archive Disk");

  await expect
    .element(screen.getByText(t("manage.repositories.risks.removableStorage"), { exact: true }))
    .toBeVisible();
  await expect
    .element(screen.getByText(t("manage.repositories.risks.nonDefaultLocation"), { exact: true }))
    .toBeVisible();
  const confirm = screen.getByRole("button", {
    name: t("manage.repositories.hostAction.confirmRisk"),
    exact: true,
  });
  await expect.element(confirm).toBeDisabled();
  await screen
    .getByText(t("manage.repositories.createRiskConfirmationTitle"), { exact: true })
    .click();
  await expect.element(confirm).toBeEnabled();
  await confirm.click();

  await expect
    .poll(() => native.resolved)
    .toEqual([
      {
        resolution: "confirm_risk",
        acknowledged_risks: ["removable_storage", "non_default_location"],
      },
    ]);
  await expect
    .element(screen.getByText(t("manage.repositories.hostAction.completed"), { exact: true }))
    .toBeVisible();
  expect(native.created[0]?.body).toMatchObject({
    kind: "authorize_storage_location",
    name: "Archive Disk",
  });
});

// An expired request is a normal outcome: the modal says so, keeps the form,
// and the restart sends a new request under a new idempotency key.
test("an expired Desktop request keeps the form and can be sent again", async () => {
  mockStoragePanelApis();
  const native = mockNativeTask({
    kind: "authorize_storage_location",
    status: "expired",
    problem: {
      type: "https://lumilio.org/problems/storage/host-action-expired",
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      retryable: true,
    },
  });
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  await sendAuthorizeRequest(screen, "Archive Disk");

  await expect
    .element(screen.getByText(t("manage.repositories.hostAction.expired"), { exact: true }))
    .toBeVisible();
  expect(document.querySelector('[role="alert"].alert-error')).toBeNull();
  await screen
    .getByRole("button", { name: t("manage.repositories.hostAction.sendAgain"), exact: true })
    .click();

  const nameField = screen.getByLabelText(t("manage.repositories.hostAction.locationName"), {
    exact: true,
  });
  await expect.element(nameField).toHaveValue("Archive Disk");
  native.set({ status: "pending", problem: undefined });
  await screen
    .getByRole("button", { name: t("manage.repositories.hostAction.send"), exact: true })
    .click();

  await expect.poll(() => native.created.length).toBe(2);
  expect(native.created[1]?.idempotencyKey).toBeTruthy();
  expect(native.created[1]?.idempotencyKey).not.toBe(native.created[0]?.idempotencyKey);
  expect(native.created[1]?.body).toMatchObject({ name: "Archive Disk" });
});

// A reconnect the role-aware policy withholds reaches the Web with its
// blockers, rendered by the same presenter as the HTTP command, not as a
// generic Desktop failure.
test("a withheld native reconnect shows the policy blocker, not a generic failure", async () => {
  mockStoragePanelApis();
  mockNativeTask({
    kind: "locate_repository",
    repository_id: "repo-offline",
    status: "failed",
    problem: {
      type: "https://lumilio.org/problems/storage/action-unavailable",
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      retryable: false,
    },
    refusal: {
      action: "reconnect",
      blockers: ["repository_busy"],
      alternatives: ["wait_for_idle"],
    },
  });
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);
  await expect.element(screen.getByText("Client Projects", { exact: true })).toBeVisible();

  await screen
    .getByRole("button", {
      name: t("storagePanel.actionsFor", {
        defaultValue: "Actions for {{name}}",
        name: "Client Projects",
      }),
      exact: true,
    })
    .click();
  await screen
    .getByRole("menuitem", { name: t("storagePanel.action.reconnect"), exact: true })
    .click();
  await screen
    .getByRole("button", { name: t("manage.repositories.hostAction.send"), exact: true })
    .click();

  await expect
    .element(
      screen.getByText(t("apiErrors.storage.actionUnavailable.repositoryBusy"), { exact: true }),
    )
    .toBeVisible();
  await expect
    .element(
      screen.getByRole("button", {
        name: t("manage.repositories.hostAction.sendAgain"),
        exact: true,
      }),
    )
    .toBeVisible();
});

// R9: an external Storage Location on a Desktop host carries its direct
// Reconnect entry. While the Server withholds it the entry is disabled and
// carries the Server's reason, so the prerequisite is visible; the Default
// Storage Location never offers it.
test("a withheld Storage Location Reconnect shows the Server's reason on the entry", async () => {
  mockStoragePanelApis();
  mockNativeTask({ kind: "locate_storage_location", status: "pending" });
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  await screen
    .getByRole("button", {
      name: t("storagePanel.locationActions", {
        defaultValue: "Actions for {{name}}",
        name: "Archive Disk",
      }),
      exact: true,
    })
    .click();
  const reconnect = screen.getByRole("menuitem", {
    name: t("storagePanel.action.reconnectLocation", {
      defaultValue: "Reconnect Storage Location",
    }),
    exact: true,
  });
  await expect.element(reconnect).toBeDisabled();
  expect(reconnect.element().closest("li")?.getAttribute("data-tip")).toBe(
    t("apiErrors.storage.actionUnavailable.originalAvailable"),
  );

  await screen
    .getByRole("button", {
      name: t("storagePanel.locationActions", {
        defaultValue: "Actions for {{name}}",
        name: t("productTerms.defaultStorageLocation"),
      }),
      exact: true,
    })
    .click();
  await expect
    .element(
      screen.getByRole("menuitem", {
        name: t("storagePanel.action.removeLocation", { defaultValue: "Remove Storage Location" }),
        exact: true,
      }),
    )
    .toBeVisible();
  await expect
    .element(
      screen.getByRole("menuitem", {
        name: t("storagePanel.action.reconnectLocation", {
          defaultValue: "Reconnect Storage Location",
        }),
        exact: true,
      }),
    )
    .not.toBeInTheDocument();
});

// R9: when the Server reports the Location's original as positively
// unavailable, the entry is live and starts a locate_storage_location task
// for exactly that Location, with the Reconnect wording on the request.
test("an unavailable external Storage Location reconnects through a native Location task", async () => {
  mockStoragePanelApis();
  worker.use(
    http.get("*/api/v1/storage/view", () =>
      HttpResponse.json({
        ...storageViewFixture,
        storage_locations: storageViewFixture.storage_locations.map((location) =>
          location.id === "loc-archive"
            ? {
                ...location,
                actions: ["reconnect"],
                unavailable_actions: [
                  { action: "detach", blockers: ["registered_repositories"], alternatives: [] },
                ],
              }
            : location,
        ),
      }),
    ),
  );
  const native = mockNativeTask({ kind: "locate_storage_location", status: "pending" });
  await page.viewport(1280, 1000);
  const screen = await renderWithProviders(<StoragePanel />);
  await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();

  await screen
    .getByRole("button", {
      name: t("storagePanel.locationActions", {
        defaultValue: "Actions for {{name}}",
        name: "Archive Disk",
      }),
      exact: true,
    })
    .click();
  const reconnect = screen.getByRole("menuitem", {
    name: t("storagePanel.action.reconnectLocation", {
      defaultValue: "Reconnect Storage Location",
    }),
    exact: true,
  });
  await expect.element(reconnect).toBeEnabled();
  await reconnect.click();
  await expect
    .element(
      screen.getByRole("heading", {
        name: t("manage.repositories.hostAction.locateLocation"),
        exact: true,
      }),
    )
    .toBeVisible();
  await screen
    .getByRole("button", { name: t("manage.repositories.hostAction.send"), exact: true })
    .click();
  await expect.poll(() => native.created.length).toBe(1);
  expect(native.created[0]?.body).toMatchObject({
    kind: "locate_storage_location",
    storage_location_id: "loc-archive",
  });
});

type RecoveryResponse = components["schemas"]["dto.RepositoryRecoveryDTO"];
type RecoveryItem = components["schemas"]["dto.RepositoryRecoveryItemDTO"];

const stagingItem = {
  id: "11111111-1111-4111-8111-111111111111",
  receipt_id: "22222222-2222-4222-8222-222222222222",
  class: "staging",
  path: ".lumilio/staging/incoming/abandoned.part",
  size: 27,
  disposition: "moved",
  present: true,
  actions: ["export", "delete"],
} satisfies RecoveryItem;

const studioItem = {
  id: "33333333-3333-4333-8333-333333333333",
  receipt_id: "22222222-2222-4222-8222-222222222222",
  class: "studio",
  path: ".lumilio/sidecars/old.lumilio-sidecar",
  size: 310,
  disposition: "reassociated",
  present: true,
  actions: [],
  resolved_asset_id: "44444444-4444-4444-8444-444444444444",
} satisfies RecoveryItem;

function recoveryFixture(deleted: string[]): RecoveryResponse {
  return {
    repository_id: "repo-family",
    receipts: [
      {
        id: "22222222-2222-4222-8222-222222222222",
        event: "open",
        recorded_at: "2026-10-18T10:00:00Z",
        actor: "web:user:1",
        trash_files: 2,
        trash_bytes: 4096,
        log_files: 2,
        derived_files: 0,
      },
    ],
    items: [
      deleted.includes(stagingItem.id)
        ? { ...stagingItem, disposition: "deleted", present: false, actions: [] }
        : stagingItem,
      studioItem,
    ],
    trash: { held_count: 2, held_bytes: 4096 },
    unreadable_receipts: [],
  };
}

// P4 / R4 Web flow: the recovery inventory is reachable from the row, names
// each preserved file with its class and what became of it, exports one
// without changing it, and deletes one only after an explicit confirmation
// that the inventory then records. A reassociated Studio record offers no
// destructive action.
test("the recovery inventory exports and deletes preserved files explicitly", async () => {
  const deleted: string[] = [];
  const exported: string[] = [];
  worker.use(
    http.get("*/api/v1/storage/repositories/:id/recovery", () =>
      HttpResponse.json(recoveryFixture(deleted)),
    ),
    http.get("*/api/v1/storage/repositories/:id/recovery/items/:item/content", ({ params }) => {
      exported.push(String(params.item));
      return new HttpResponse("uncommitted incoming bytes\n", {
        headers: { "Content-Type": "application/octet-stream" },
      });
    }),
    http.delete("*/api/v1/storage/repositories/:id/recovery/items/:item", ({ params }) => {
      deleted.push(String(params.item));
      return HttpResponse.json({
        ...stagingItem,
        disposition: "deleted",
        present: false,
        actions: [],
      });
    }),
  );
  const downloads: string[] = [];
  const captureDownload = (event: MouseEvent) => {
    if (!(event.target instanceof HTMLAnchorElement) || !event.target.download) return;
    event.preventDefault();
    downloads.push(event.target.download);
  };
  document.addEventListener("click", captureDownload);
  try {
    const screen = await renderPanel();
    await expect.element(screen.getByText("Family Archive", { exact: true })).toBeVisible();
    // The Trash column names the held files beside the ordinary count.
    await expect
      .element(screen.getByText(t("storagePanel.trashHeld", { count: 2 }), { exact: false }))
      .toBeVisible();

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
      .getByRole("menuitem", { name: t("storagePanel.action.recovery"), exact: true })
      .click();

    await expect
      .element(screen.getByRole("heading", { name: t("storagePanel.recovery.title"), exact: true }))
      .toBeVisible();
    await expect.element(screen.getByText("abandoned.part", { exact: true })).toBeVisible();
    await expect
      .element(screen.getByText(t("storagePanel.recovery.disposition.moved"), { exact: false }))
      .toBeVisible();
    await expect
      .element(
        screen.getByText(t("storagePanel.recovery.disposition.reassociated"), { exact: false }),
      )
      .toBeVisible();
    await expect
      .element(
        screen.getByRole("link", {
          name: t("storagePanel.recovery.heldTrashValue", { count: 2 }),
          exact: true,
        }),
      )
      .toBeVisible();
    // The reassociated Studio record is live state: no delete, no export.
    await expect
      .element(
        screen.getByRole("button", {
          name: t("storagePanel.recovery.deleteItem", { name: "old.lumilio-sidecar" }),
          exact: true,
        }),
      )
      .not.toBeInTheDocument();

    await screen
      .getByRole("button", {
        name: t("storagePanel.recovery.exportItem", { name: "abandoned.part" }),
        exact: true,
      })
      .click();
    await expect.poll(() => exported).toEqual([stagingItem.id]);
    await expect.poll(() => downloads).toEqual(["abandoned.part"]);
    expect(deleted).toEqual([]);

    await screen
      .getByRole("button", {
        name: t("storagePanel.recovery.deleteItem", { name: "abandoned.part" }),
        exact: true,
      })
      .click();
    await expect
      .element(
        screen.getByText(t("storagePanel.recovery.deleteWarning", { name: "abandoned.part" })),
      )
      .toBeVisible();
    expect(deleted).toEqual([]);
    await screen
      .getByRole("button", { name: t("storagePanel.recovery.confirmDelete"), exact: true })
      .click();
    await expect.poll(() => deleted).toEqual([stagingItem.id]);
    await expect
      .element(
        screen.getByText(
          `${t("storagePanel.recovery.class.staging")} · ${t("storagePanel.recovery.disposition.deleted")}`,
          { exact: false },
        ),
      )
      .toBeVisible();
    await expect
      .element(
        screen.getByRole("button", {
          name: t("storagePanel.recovery.exportItem", { name: "abandoned.part" }),
          exact: true,
        }),
      )
      .not.toBeInTheDocument();
  } finally {
    document.removeEventListener("click", captureDownload);
  }
});
