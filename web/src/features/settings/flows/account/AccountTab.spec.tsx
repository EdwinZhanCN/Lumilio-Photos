import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { seedSession } from "@test/session";
import { t } from "@test/i18n";
import AccountTab from "./AccountTab";

// UserAvatar (presentational) and PhotoPicker (the heavy asset browser, only
// mounted when picking an avatar) are the boundaries. The real auth session,
// MFA/passkey queries and profile mutation all run against MSW.
vi.mock("@/components/ui/UserAvatar", () => ({
  default: ({ name }: { name?: string }) => <div data-testid="user-avatar">{name}</div>,
}));
vi.mock("@/features/assets/picker", () => ({ default: () => <div>photo-picker</div> }));

type ProfileUser = {
  user_id: number;
  username: string;
  display_name?: string;
  avatar_asset_id?: string;
  role: string;
  permissions?: string[];
};

/** Seed the session and the account cards' bootstrap queries, then render. */
async function renderAccount(user: ProfileUser) {
  seedSession(user);
  worker.use(
    http.get("*/api/v1/auth/mfa", () =>
      HttpResponse.json({ totp_enabled: false, recovery_codes_remaining: 0 }),
    ),
    http.get("*/api/v1/auth/mfa/passkeys", () => HttpResponse.json({ credentials: [] })),
  );
  return renderWithProviders(<AccountTab />, { auth: true });
}

const alex: ProfileUser = {
  user_id: 1,
  username: "alex",
  display_name: "Old Name",
  role: "user",
  permissions: ["manage_own_profile"],
};

describe("AccountTab profile", () => {
  it("saves an edited display name and confirms success", async () => {
    let body: unknown;
    worker.use(
      http.patch("*/api/v1/users/me/profile", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...alex, display_name: "New Name" });
      }),
    );

    const screen = await renderAccount(alex);

    await screen.getByLabelText(t("settings.account.displayName"), { exact: true }).fill("New Name");
    await screen.getByRole("button", { name: t("settings.section.save") }).click();

    await expect.element(screen.getByText(t("settings.section.saved"))).toBeVisible();
    expect(body).toEqual({ display_name: "New Name", avatar_asset_id: "" });
  });

  it("surfaces an error and keeps the session when the server returns an invalid payload", async () => {
    worker.use(
      http.patch("*/api/v1/users/me/profile", () => HttpResponse.json({ user_id: undefined })),
    );

    const screen = await renderAccount(alex);

    await screen.getByLabelText(t("settings.account.displayName"), { exact: true }).fill("New Name");
    await screen.getByRole("button", { name: t("settings.section.save") }).click();

    await expect
      .element(screen.getByText(t("settings.account.invalidProfileResponse")))
      .toBeVisible();
  });

  it("submits an explicit empty avatar value when clearing the avatar", async () => {
    let body: unknown;
    worker.use(
      http.patch("*/api/v1/users/me/profile", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...alex, avatar_asset_id: undefined });
      }),
    );

    const screen = await renderAccount({ ...alex, avatar_asset_id: "asset-1" });

    await screen.getByRole("button", { name: t("settings.account.removeAvatar") }).click();
    await screen.getByRole("button", { name: t("settings.section.save") }).click();

    await vi.waitFor(() => {
      expect(body).toEqual({ display_name: "Old Name", avatar_asset_id: "" });
    });
  });
});
