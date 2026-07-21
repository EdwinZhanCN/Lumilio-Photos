import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { seedSession } from "@test/session";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import UsersTab from "./UsersTab";

type ManagedUserDTO = components["schemas"]["dto.ManagedUserDTO"];

// UserAvatar (presentational) and PhotoPicker (the heavy asset browser, only
// mounted when picking an avatar) are the boundaries of this flow spec. The real
// auth bootstrap, users query and admin-update mutation all run against MSW.
vi.mock("@/components/ui/UserAvatar", () => ({
  default: ({ name, assetId }: { name?: string; assetId?: string }) => (
    <div data-testid="user-avatar">
      {name}:{assetId ?? "initials"}
    </div>
  ),
}));
vi.mock("@/features/assets/picker", () => ({ default: () => <div>photo-picker</div> }));

const alex = {
  user_id: 2,
  username: "alex",
  display_name: "Alex",
  avatar_asset_id: "asset-1",
  role: "user",
  is_active: true,
  asset_count: 0,
  album_count: 0,
} satisfies ManagedUserDTO;

describe("UsersTab", () => {
  beforeEach(() => {
    seedSession({ user_id: 1, username: "admin", role: "admin" });
  });

  it("submits an explicit empty avatar value when an admin clears a user avatar", async () => {
    let updateBody: unknown;
    worker.use(
      http.get("*/api/v1/users", () => HttpResponse.json({ users: [alex], total: 1 })),
      http.patch("*/api/v1/users/:id", async ({ request }) => {
        updateBody = await request.json();
        return HttpResponse.json({ ...alex, avatar_asset_id: "" });
      }),
    );

    const screen = await renderWithProviders(<UsersTab />, { router: false, auth: true });

    await screen.getByRole("button", { name: t("settings.users.removeAvatar") }).click();
    await screen.getByRole("button", { name: t("settings.users.save") }).click();

    await vi.waitFor(() => {
      expect(updateBody).toEqual({
        username: "alex",
        display_name: "Alex",
        avatar_asset_id: "",
        role: "user",
        is_active: true,
      });
    });
  });
});
