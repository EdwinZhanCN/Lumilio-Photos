import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { StorageMonitor } from "./StorageMonitor";

/**
 * Spec-owned fixture value. Legacy server names stay literal on purpose: this
 * assertion proves that semantic identity wins over a raw `name`, so it must
 * never be resolved through i18n. Every other literal in this file is either a
 * fixture value or a filesystem path.
 */
const LEGACY_REPOSITORY_NAME = "Primary";

describe("StorageMonitor", () => {
  it("groups repositories beneath their owning Storage Locations", async () => {
    worker.use(
      http.get("*/api/v1/repositories/storage-diagnostics", () =>
        HttpResponse.json({
          generated_at: "2026-08-09T00:00:00Z",
          items: [
            {
              ...diagnostic("storage_location", "root-primary", "legacy default name", "/storage"),
              kind: "default",
            },
            diagnostic("storage_location", "root-archive", "Archive Disk", "/archive"),
            {
              ...diagnostic("repository", "repo-family", "Family Archive", "/archive/family"),
              parent_target_id: "root-archive",
            },
            {
              ...diagnostic(
                "repository",
                "repo-primary",
                LEGACY_REPOSITORY_NAME,
                "/storage/primary",
              ),
              parent_target_id: "root-primary",
              role: "primary",
            },
          ],
        }),
      ),
    );

    const screen = await renderWithProviders(<StorageMonitor />);

    // 标题不重复：Tab 名由路由表头承担，快照头只剩上次成功时间与操作
    await expect
      .element(screen.getByRole("heading", { name: t("monitor.tabs.storage"), exact: true }))
      .not.toBeInTheDocument();
    // 全部健康时不作声：既没有关注计数，也没有报平安的句子
    await expect
      .element(
        screen.getByText(t("monitor.storage.attentionNeeded", { count: 1 }), { exact: true }),
      )
      .not.toBeInTheDocument();

    const primaryDetail = screen.getByRole("region", {
      name: t("productTerms.defaultStorageLocation"),
    });
    await expect.element(primaryDetail).toHaveClass(/h-auto/);
    const map = primaryDetail.getByRole("group", { name: t("monitor.storage.capacityHeading") });
    await expect
      .element(map.getByRole("button", { name: t("monitor.storage.statUsed"), exact: false }))
      .toHaveAttribute("aria-pressed", "true");
    const repositoryList = primaryDetail.getByRole("list");
    await expect.element(repositoryList).toHaveClass(/h-auto/);
    await expect.element(repositoryList).toHaveClass(/overflow-y-auto/);
    await expect.element(repositoryList).toHaveClass(/border-0/);
    await expect
      .element(primaryDetail.getByText(t("monitor.storage.technicalDetails"), { exact: true }))
      .toBeVisible();
    // 容量告警留在默认收起的诊断里，不占首屏篇幅
    await expect
      .element(primaryDetail.getByText(t("monitor.storage.capacityNote"), { exact: true }))
      .not.toBeVisible();
    await expect
      .element(
        primaryDetail.getByText(t("monitor.storage.totalCapacity", { total: "976.56 KB" }), {
          exact: false,
        }),
      )
      .toBeVisible();
    await expect
      .element(primaryDetail.getByText("/storage/primary", { exact: true }))
      .toBeVisible();
    await expect
      .element(
        primaryDetail.getByRole("button", {
          name: `${t("common.copy")} ${t("productTerms.primaryRepository")}`,
          exact: true,
        }),
      )
      .toBeVisible();
    expect(primaryDetail.element()?.scrollWidth).toBeLessThanOrEqual(
      primaryDetail.element()?.clientWidth ?? 0,
    );

    const nav = screen.getByRole("navigation", { name: t("monitor.storage.navLabel") });
    await expect
      .element(nav.getByRole("button", { name: "Family Archive", exact: true }))
      .toBeVisible();
    await expect
      .element(nav.getByRole("button", { name: t("productTerms.primaryRepository"), exact: true }))
      .toBeVisible();

    await nav
      .getByRole("button", { name: t("productTerms.primaryRepository"), exact: true })
      .click();
    const repositoryDetail = screen.getByRole("region", {
      name: t("productTerms.primaryRepository"),
      exact: true,
    });
    await expect
      .element(
        repositoryDetail.getByRole("heading", { name: t("monitor.storage.capacityHeading") }),
      )
      .toBeVisible();
    await expect
      .element(repositoryDetail.getByText(t("monitor.storage.technicalDetails"), { exact: true }))
      .toBeVisible();
    await expect.element(repositoryDetail.getByRole("list")).not.toBeInTheDocument();

    // 点击行主体只选中：显示 Archive Disk 的详情，不改变展开状态
    await nav.getByRole("button", { name: "Archive Disk", exact: true }).click();
    const archiveDetail = screen.getByRole("region", { name: "Archive Disk" });
    await expect.element(archiveDetail.getByText("Family Archive", { exact: true })).toBeVisible();
    // 语义身份优先于服务器原始名称：rawName 不得作为显示名出现
    await expect
      .element(archiveDetail.getByText(LEGACY_REPOSITORY_NAME, { exact: true }))
      .not.toBeInTheDocument();
  });

  it("expands and collapses a location only through its chevron", async () => {
    worker.use(
      http.get("*/api/v1/repositories/storage-diagnostics", () =>
        HttpResponse.json({
          generated_at: "2026-08-09T00:00:00Z",
          items: [
            {
              ...diagnostic("storage_location", "root-primary", "Primary Storage", "/storage"),
              kind: "default",
            },
            {
              ...diagnostic(
                "repository",
                "repo-primary",
                LEGACY_REPOSITORY_NAME,
                "/storage/primary",
              ),
              parent_target_id: "root-primary",
              role: "primary",
            },
          ],
        }),
      ),
    );

    const screen = await renderWithProviders(<StorageMonitor />);
    const nav = screen.getByRole("navigation", { name: t("monitor.storage.navLabel") });
    const repoRow = nav.getByRole("button", {
      name: t("productTerms.primaryRepository"),
      exact: true,
    });
    const locationRow = nav.getByRole("button", {
      name: t("productTerms.defaultStorageLocation"),
      exact: true,
    });
    await expect.element(repoRow).toBeVisible();
    await expect.element(locationRow).toHaveAttribute("aria-expanded", "true");

    // 点击行首 chevron 图标收起：子项消失
    const chevron = locationRow.element()?.querySelector("[data-chevron]");
    chevron?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await expect.element(repoRow).not.toBeInTheDocument();
    await expect.element(locationRow).toHaveAttribute("aria-expanded", "false");

    // 点击行主体：只选中详情，不重新展开
    await locationRow.click();
    await expect
      .element(
        screen.getByRole("region", {
          name: t("productTerms.defaultStorageLocation"),
        }),
      )
      .toBeVisible();
    await expect.element(repoRow).not.toBeInTheDocument();

    // 再次点击 chevron 展开
    const chevronAgain = locationRow.element()?.querySelector("[data-chevron]");
    chevronAgain?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await expect.element(repoRow).toBeVisible();
  });

  it("keeps repositories without a parent identity in an attention section", async () => {
    worker.use(
      http.get("*/api/v1/repositories/storage-diagnostics", () =>
        HttpResponse.json({
          generated_at: "2026-08-09T00:00:00Z",
          items: [diagnostic("repository", "repo-orphan", "Detached Archive", "/lost/archive")],
        }),
      ),
    );

    const screen = await renderWithProviders(<StorageMonitor />);

    await expect
      .element(
        screen.getByText(t("monitor.storage.unlinkedTitle"), {
          exact: true,
        }),
      )
      .toBeVisible();
    await expect.element(screen.getByRole("heading", { name: "Detached Archive" })).toBeVisible();
  });
});

function diagnostic(targetType: string, targetID: string, name: string, path: string) {
  return {
    target_type: targetType,
    target_id: targetID,
    name,
    path,
    canonical_path: path,
    reachability: "active",
    writable: true,
    capacity_known: true,
    total_bytes: 1_000_000,
    available_bytes: 600_000,
    marker_uuid: "a7b13458-c100-4149-8e4e-670d705ea227",
    risk_warnings: [],
  };
}

it("keeps read-only and mount risks visible, with stale data on a failed manual refresh", async () => {
  let failed = false;
  worker.use(
    http.get("*/api/v1/repositories/storage-diagnostics", () =>
      failed
        ? HttpResponse.json({ title: "Fixture failure" }, { status: 500 })
        : HttpResponse.json({
            items: [
              {
                ...diagnostic("storage_location", "fixture-root", "Fixture disk", "/fixture"),
                writable: false,
                mount_fingerprint_changed: true,
                risk_warnings: ["mount_fingerprint_changed"],
              },
            ],
          }),
    ),
  );
  const screen = await renderWithProviders(<StorageMonitor />);
  await expect
    .element(screen.getByText(t("monitor.storage.readOnly"), { exact: true }))
    .toBeVisible();
  await expect
    .element(screen.getByText(t("monitor.storage.riskMountChanged"), { exact: true }))
    .toBeVisible();
  await expect
    .element(screen.getByText(t("monitor.storage.attentionNeeded", { count: 1 }), { exact: true }))
    .toBeVisible();
  failed = true;
  await screen
    .getByRole("button", { name: t("settings.serverSettings.refresh"), exact: true })
    .click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.snapshot.stale"));
  await expect.element(screen.getByRole("heading", { name: "Fixture disk" })).toBeVisible();
  // 陈旧快照不得断言当前的关注状态
  await expect
    .element(screen.getByText(t("monitor.storage.attentionNeeded", { count: 1 }), { exact: true }))
    .not.toBeInTheDocument();
});
