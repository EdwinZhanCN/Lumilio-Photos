import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import AccountTab from "./AccountTab";

const mocks = vi.hoisted(() => ({
  dispatch: vi.fn(),
  navigate: vi.fn(),
  updateProfileMutateAsync: vi.fn(),
  beginPasskeyMutateAsync: vi.fn(),
  verifyPasskeyMutateAsync: vi.fn(),
  deletePasskeyMutateAsync: vi.fn(),
  user: {
    user_id: 1,
    username: "alex",
    display_name: "Old Name",
    avatar_asset_id: undefined as string | undefined,
    role: "user",
    permissions: ["manage_own_profile"],
  },
  mfaStatusQuery: {
    data: {
      data: {
        totp_enabled: false,
        recovery_codes_remaining: 0,
      },
    },
    isLoading: false,
    isFetching: false,
    isError: false,
  },
  passkeysQuery: {
    passkeys: [] as Array<{ passkey_id?: number; label?: string }>,
    isLoading: false,
    isFetching: false,
    isError: false,
  },
}));

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key,
  }),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => mocks.navigate,
}));

vi.mock("@/features/auth", () => ({
  useAuth: () => ({
    user: mocks.user,
    dispatch: mocks.dispatch,
  }),
}));

vi.mock("@/features/auth/hooks/useMFA.ts", () => ({
  useMFAStatus: () => mocks.mfaStatusQuery,
}));

vi.mock("@/features/auth/hooks/usePasskeys.ts", () => ({
  usePasskeys: () => mocks.passkeysQuery,
  useBeginPasskeyEnrollment: () => ({
    mutateAsync: mocks.beginPasskeyMutateAsync,
    isPending: false,
  }),
  useVerifyPasskeyEnrollment: () => ({
    mutateAsync: mocks.verifyPasskeyMutateAsync,
    isPending: false,
  }),
  useDeletePasskey: () => ({
    mutateAsync: mocks.deletePasskeyMutateAsync,
    isPending: false,
  }),
}));

vi.mock("@/features/users/hooks/useUsers", () => ({
  useUpdateMyProfile: () => ({
    mutateAsync: mocks.updateProfileMutateAsync,
    isPending: false,
  }),
}));

vi.mock("@/features/auth/lib/webauthn.ts", () => ({
  createPasskeyCredential: vi.fn(),
  getPasskeySupport: () => ({ supported: true }),
}));

vi.mock("@/components/UserAvatar", () => ({
  default: ({ name, assetId }: { name?: string; assetId?: string }) => (
    <div data-testid="user-avatar">
      {name}:{assetId ?? "initials"}
    </div>
  ),
}));

vi.mock("@/components/PhotoPicker", () => ({
  default: () => <div>photo-picker</div>,
}));

describe("AccountTab", () => {
  beforeEach(() => {
    mocks.dispatch.mockClear();
    mocks.navigate.mockClear();
    mocks.updateProfileMutateAsync.mockReset();
    mocks.user = {
      user_id: 1,
      username: "alex",
      display_name: "Old Name",
      avatar_asset_id: undefined,
      role: "user",
      permissions: ["manage_own_profile"],
    };
    mocks.mfaStatusQuery = {
      data: {
        data: {
          totp_enabled: false,
          recovery_codes_remaining: 0,
        },
      },
      isLoading: false,
      isFetching: false,
      isError: false,
    };
    mocks.passkeysQuery = {
      passkeys: [],
      isLoading: false,
      isFetching: false,
      isError: false,
    };
  });

  afterEach(() => {
    cleanup();
  });

  it("saves display name without clearing the authenticated session", async () => {
    const updatedUser = {
      ...mocks.user,
      display_name: "New Name",
    };
    mocks.updateProfileMutateAsync.mockResolvedValue({
      code: 0,
      message: "success",
      data: updatedUser,
    });

    render(<AccountTab />);

    fireEvent.change(screen.getByDisplayValue("Old Name"), {
      target: { value: "New Name" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(mocks.dispatch).toHaveBeenCalledWith({
        type: "SET_USER",
        payload: updatedUser,
      });
    });
  });

  it("does not show the unsaved changes bar before the profile is edited", () => {
    render(<AccountTab />);

    expect(
      screen.getByText("Careful, you have unsaved changes.").closest("[aria-hidden]"),
    ).toHaveAttribute("aria-hidden", "true");
  });

  it("shows an error and does not dispatch SET_USER for invalid profile payloads", async () => {
    mocks.updateProfileMutateAsync.mockResolvedValue({
      code: 0,
      message: "success",
      data: {},
    });

    render(<AccountTab />);

    fireEvent.change(screen.getByDisplayValue("Old Name"), {
      target: { value: "New Name" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(
      await screen.findByText("Profile saved, but the server returned an invalid user payload."),
    ).toBeInTheDocument();
    expect(mocks.dispatch).not.toHaveBeenCalled();
  });

  it("submits an explicit empty avatar value when clearing the avatar", async () => {
    mocks.user = {
      ...mocks.user,
      avatar_asset_id: "asset-1",
    };
    mocks.updateProfileMutateAsync.mockResolvedValue({
      code: 0,
      message: "success",
      data: {
        ...mocks.user,
        avatar_asset_id: undefined,
      },
    });

    render(<AccountTab />);

    fireEvent.click(screen.getByRole("button", { name: "Remove photo" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(mocks.updateProfileMutateAsync).toHaveBeenCalledWith({
        body: {
          display_name: "Old Name",
          avatar_asset_id: "",
        },
      });
    });
  });

  it("does not navigate from the MFA card while MFA status is loading", () => {
    mocks.mfaStatusQuery = {
      data: undefined as any,
      isLoading: true,
      isFetching: true,
      isError: false,
    };

    render(<AccountTab />);

    fireEvent.click(screen.getByRole("button", { name: /Authenticator App/i }));

    expect(mocks.navigate).not.toHaveBeenCalled();
  });

  it("does not show Not enrolled while passkeys are loading", () => {
    mocks.passkeysQuery = {
      passkeys: [],
      isLoading: true,
      isFetching: true,
      isError: false,
    };

    render(<AccountTab />);

    expect(screen.queryByText("Not enrolled")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Passkeys/i })).toBeDisabled();
  });
});
