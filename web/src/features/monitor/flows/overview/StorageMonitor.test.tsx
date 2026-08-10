import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { StorageMonitor } from "./StorageMonitor";

describe("StorageMonitor", () => {
  it("groups repositories beneath their owning Storage Locations", async () => {
    worker.use(
      http.get("*/api/v1/repositories/storage-diagnostics", () =>
        HttpResponse.json({
          generated_at: "2026-08-09T00:00:00Z",
          items: [
            diagnostic("storage_location", "root-primary", "Primary Storage", "/storage"),
            diagnostic("storage_location", "root-archive", "Archive Disk", "/archive"),
            {
              ...diagnostic("repository", "repo-family", "Family Archive", "/archive/family"),
              parent_target_id: "root-archive",
            },
            {
              ...diagnostic("repository", "repo-primary", "Primary", "/storage/primary"),
              parent_target_id: "root-primary",
            },
          ],
        }),
      ),
    );

    const screen = await renderWithProviders(<StorageMonitor />);

    const primaryDetail = screen.getByRole("region", { name: "Primary Storage" });
    await expect.element(primaryDetail).toHaveClass(/h-auto/);
    const capacityProgress = primaryDetail.getByRole("progressbar", { name: "Used capacity" });
    await expect.element(capacityProgress).toHaveAttribute("aria-valuenow", "40");
    const repositoryList = primaryDetail.getByRole("list");
    await expect.element(repositoryList).toHaveClass(/h-auto/);
    await expect.element(repositoryList).toHaveClass(/overflow-y-auto/);
    await expect.element(repositoryList).toHaveClass(/border-0/);
    await expect
      .element(primaryDetail.getByRole("heading", { name: "Technical details" }))
      .toBeVisible();
    await expect.element(primaryDetail.getByText("Total 976.56 KB", { exact: true })).toBeVisible();
    await expect
      .element(primaryDetail.getByText("/storage/primary", { exact: true }))
      .toBeVisible();
    await expect
      .element(primaryDetail.getByRole("button", { name: "Copy Primary", exact: true }))
      .toBeVisible();
    expect(primaryDetail.element()?.scrollWidth).toBeLessThanOrEqual(
      primaryDetail.element()?.clientWidth ?? 0,
    );

    const nav = screen.getByRole("navigation", { name: "Storage targets" });
    await expect
      .element(nav.getByRole("button", { name: "Family Archive", exact: true }))
      .toBeVisible();
    await expect.element(nav.getByRole("button", { name: "Primary", exact: true })).toBeVisible();

    await nav.getByRole("button", { name: "Primary", exact: true }).click();
    const repositoryDetail = screen.getByRole("region", { name: "Primary", exact: true });
    await expect.element(repositoryDetail.getByRole("heading", { name: "Capacity" })).toBeVisible();
    await expect
      .element(repositoryDetail.getByRole("heading", { name: "Technical details" }))
      .toBeVisible();
    await expect.element(repositoryDetail.getByRole("list")).not.toBeInTheDocument();

    // 点击行主体只选中：显示 Archive Disk 的详情，不改变展开状态
    await nav.getByRole("button", { name: "Archive Disk", exact: true }).click();
    const archiveDetail = screen.getByRole("region", { name: "Archive Disk" });
    await expect.element(archiveDetail.getByText("Family Archive", { exact: true })).toBeVisible();
    await expect
      .element(archiveDetail.getByText("Primary", { exact: true }))
      .not.toBeInTheDocument();
  });

  it("expands and collapses a location only through its chevron", async () => {
    worker.use(
      http.get("*/api/v1/repositories/storage-diagnostics", () =>
        HttpResponse.json({
          generated_at: "2026-08-09T00:00:00Z",
          items: [
            diagnostic("storage_location", "root-primary", "Primary Storage", "/storage"),
            {
              ...diagnostic("repository", "repo-primary", "Primary", "/storage/primary"),
              parent_target_id: "root-primary",
            },
          ],
        }),
      ),
    );

    const screen = await renderWithProviders(<StorageMonitor />);
    const nav = screen.getByRole("navigation", { name: "Storage targets" });
    const repoRow = nav.getByRole("button", { name: "Primary", exact: true });
    const locationRow = nav.getByRole("button", { name: "Primary Storage", exact: true });
    await expect.element(repoRow).toBeVisible();
    await expect.element(locationRow).toHaveAttribute("aria-expanded", "true");

    // 点击行首 chevron 图标收起：子项消失
    const chevron = locationRow.element()?.querySelector("[data-chevron]");
    chevron?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await expect.element(repoRow).not.toBeInTheDocument();
    await expect.element(locationRow).toHaveAttribute("aria-expanded", "false");

    // 点击行主体：只选中详情，不重新展开
    await locationRow.click();
    await expect.element(screen.getByRole("region", { name: "Primary Storage" })).toBeVisible();
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
      .element(screen.getByText("Repositories without a known location", { exact: true }))
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
