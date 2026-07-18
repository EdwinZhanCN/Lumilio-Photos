import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import RequiredPasswordChangeFlow from "./RequiredPasswordChangeFlow";
import {
  clearRequiredPasswordChangeChallenge,
  storeRequiredPasswordChangeChallenge,
} from "../../state/passwordChangeChallenge.ts";

const mocks = vi.hoisted(() => ({
  completeAuth: vi.fn(),
  mutateAsync: vi.fn(),
}));

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

vi.mock("@/lib/http-commons/queryClient", () => ({
  $api: {
    useMutation: () => ({ mutateAsync: mocks.mutateAsync, isPending: false }),
  },
}));

vi.mock("../../state/useAuth.ts", () => ({
  useAuth: () => ({ completeAuth: mocks.completeAuth }),
}));

describe("RequiredPasswordChangeFlow", () => {
  beforeEach(() => {
    mocks.completeAuth.mockReset();
    mocks.mutateAsync.mockReset();
    clearRequiredPasswordChangeChallenge();
  });

  afterEach(cleanup);

  it("returns to login after refresh loses the in-memory recovery token", () => {
    render(
      <MemoryRouter initialEntries={["/password-change-required"]}>
        <Routes>
          <Route path="/password-change-required" element={<RequiredPasswordChangeFlow />} />
          <Route path="/login" element={<div>Login again</div>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("Login again")).toBeInTheDocument();
    expect(mocks.mutateAsync).not.toHaveBeenCalled();
  });

  it("exchanges the one-use token and authenticates only after password completion", async () => {
    const authResponse = {
      token: "access",
      refreshToken: "refresh",
      user: { user_id: 1, username: "admin" },
    };
    mocks.mutateAsync.mockResolvedValue(authResponse);
    mocks.completeAuth.mockResolvedValue(authResponse.user);
    storeRequiredPasswordChangeChallenge({
      passwordChangeToken: "one-use",
      username: "admin",
      redirectTo: "/done",
    });

    render(
      <MemoryRouter initialEntries={["/password-change-required"]}>
        <Routes>
          <Route path="/password-change-required" element={<RequiredPasswordChangeFlow />} />
          <Route path="/done" element={<div>Authenticated</div>} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByLabelText("New password"), {
      target: { value: "StrongPass123" },
    });
    fireEvent.change(screen.getByLabelText("Confirm new password"), {
      target: { value: "StrongPass123" },
    });
    expect(mocks.completeAuth).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Set password and continue" }));

    await waitFor(() => {
      expect(mocks.mutateAsync).toHaveBeenCalledWith({
        body: { password_change_token: "one-use", new_password: "StrongPass123" },
      });
      expect(mocks.completeAuth).toHaveBeenCalledWith(authResponse);
    });
    expect(await screen.findByText("Authenticated")).toBeInTheDocument();
  });
});
