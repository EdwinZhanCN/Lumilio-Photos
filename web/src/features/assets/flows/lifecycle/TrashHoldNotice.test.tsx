import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import TrashHoldNotice from "./TrashHoldNotice";

// P4: the Trash view names the inherited files it holds and lets the user
// accept the normal retention for the whole Repository in one confirmed
// command; without a Repository scope the decision stays with the selection.
describe("TrashHoldNotice", () => {
  it("names the held files and accepts retention for the Repository", async () => {
    let held = ["a", "b"];
    const accepted: unknown[] = [];
    worker.use(
      http.get("*/api/v1/assets/trash-holds", ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.get("repository_id")).toBe("repo-1");
        return HttpResponse.json({ asset_ids: held, count: held.length });
      }),
      http.post("*/api/v1/assets/accept-retention", async ({ request }) => {
        accepted.push(await request.json());
        held = [];
        return HttpResponse.json({ entries: 2 });
      }),
    );
    const screen = await renderWithProviders(<TrashHoldNotice repositoryId="repo-1" />);
    await expect
      .element(screen.getByText(t("assets.trash.holds.title", { count: 2 }), { exact: true }))
      .toBeVisible();
    await screen
      .getByRole("button", { name: t("assets.trash.holds.acceptAll"), exact: true })
      .click();
    await expect
      .element(screen.getByText(t("assets.trash.holds.acceptWarning"), { exact: true }))
      .toBeVisible();
    expect(accepted).toEqual([]);
    await screen.getByRole("button", { name: t("common.confirm"), exact: true }).click();
    await expect.poll(() => accepted).toEqual([{ repository_id: "repo-1", confirm: true }]);
    await expect
      .element(screen.getByText(t("assets.trash.holds.title", { count: 2 }), { exact: true }))
      .not.toBeInTheDocument();
  });

  it("renders nothing when no file is held", async () => {
    worker.use(
      http.get("*/api/v1/assets/trash-holds", () => HttpResponse.json({ asset_ids: [], count: 0 })),
    );
    const screen = await renderWithProviders(<TrashHoldNotice repositoryId="repo-1" />);
    await expect.element(screen.getByRole("status")).not.toBeInTheDocument();
  });

  it("offers no Repository-wide acceptance outside a Repository scope", async () => {
    worker.use(
      http.get("*/api/v1/assets/trash-holds", ({ request }) => {
        expect(new URL(request.url).searchParams.get("repository_id")).toBeNull();
        return HttpResponse.json({ asset_ids: ["a"], count: 1 });
      }),
    );
    const screen = await renderWithProviders(<TrashHoldNotice />);
    await expect
      .element(screen.getByText(t("assets.trash.holds.title", { count: 1 }), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByRole("button", { name: t("assets.trash.holds.acceptAll"), exact: true }))
      .not.toBeInTheDocument();
  });
});
