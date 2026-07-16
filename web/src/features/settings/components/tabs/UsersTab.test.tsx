import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import UsersTab from "./UsersTab";

const mocks = vi.hoisted(() => ({
  dispatch: vi.fn(),
  updateUserMutateAsync: vi.fn(),
  resetAccessMutateAsync: vi.fn(),
  admin: {
    user_id: 1,
    username: "admin",
    display_name: "Admin",
    role: "admin",
  },
  users: [
    {
      user_id: 2,
      username: "alex",
      display_name: "Alex",
      avatar_asset_id: "asset-1",
      role: "user",
      is_active: true,
      asset_count: 0,
      album_count: 0,
    },
  ],
}));

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key,
  }),
}));

vi.mock("@/features/auth", () => ({
  useAuth: () => ({
    user: mocks.admin,
    dispatch: mocks.dispatch,
  }),
}));

vi.mock("@/features/users/hooks/useUsers", () => ({
  useUsers: () => ({
    users: mocks.users,
    isLoading: false,
    isError: false,
  }),
  useAdminUpdateUser: () => ({
    mutateAsync: mocks.updateUserMutateAsync,
    isPending: false,
  }),
  useResetUserAccess: () => ({
    mutateAsync: mocks.resetAccessMutateAsync,
    isPending: false,
  }),
}));

vi.mock("@/components/ui/UserAvatar", () => ({
  default: ({ name, assetId }: { name?: string; assetId?: string }) => (
    <div data-testid="user-avatar">
      {name}:{assetId ?? "initials"}
    </div>
  ),
}));

vi.mock("@/components/PhotoPicker", () => ({
  default: () => <div>photo-picker</div>,
}));

describe("UsersTab", () => {
  beforeEach(() => {
    mocks.dispatch.mockClear();
    mocks.updateUserMutateAsync.mockReset();
    mocks.updateUserMutateAsync.mockResolvedValue({
      ...mocks.users[0],
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("submits an explicit empty avatar value when an admin clears a user avatar", async () => {
    render(<UsersTab />);

    fireEvent.click(await screen.findByRole("button", { name: "Remove photo" }));
    fireEvent.click(screen.getByRole("button", { name: "Save user" }));

    await waitFor(() => {
      expect(mocks.updateUserMutateAsync).toHaveBeenCalledWith({
        params: {
          path: {
            id: 2,
          },
        },
        body: {
          username: "alex",
          display_name: "Alex",
          avatar_asset_id: "",
          role: "user",
          is_active: true,
        },
      });
    });
  });
});
