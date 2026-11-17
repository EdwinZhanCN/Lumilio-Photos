import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { AssetAvailabilityNotice } from "./AssetAvailabilityNotice";

const instance = "urn:lumilio:problem:0123456789abcdef0123456789abcdef";
describe("viewer lifecycle Problems", () => {
  it.each(["missing", "offline", "trashed"] as const)(
    "shows the actionable %s state",
    async (state) => {
      const error = { type: `https://lumilio.org/problems/asset/${state}`, status: 409, instance };
      const screen = await renderWithProviders(
        <AssetAvailabilityNotice assetId="a" error={error} />,
      );
      await expect.element(screen.getByText(t(`apiErrors.asset.${state}`))).toBeVisible();
      if (state !== "trashed")
        await expect.element(screen.getByRole("button")).not.toBeInTheDocument();
    },
  );
  it("restores a trashed Asset through the complete-selection command", async () => {
    const requests: unknown[] = [];
    worker.use(
      http.post("*/api/v1/assets/restore", async ({ request }) => {
        requests.push(await request.json());
        return HttpResponse.json({ files: 1, renamed: [] });
      }),
    );
    const screen = await renderWithProviders(
      <AssetAvailabilityNotice
        assetId="a"
        error={{ type: "https://lumilio.org/problems/asset/trashed", status: 409, instance }}
      />,
    );
    await screen.getByRole("button", { name: t("assets.lifecycle.restore"), exact: true }).click();
    await expect.poll(() => requests).toEqual([{ asset_ids: ["a"] }]);
  });
});
