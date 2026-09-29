import { useEffect, type ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { seedSession } from "@test/session";
import { t } from "@test/i18n";
import ProtectedRoute from "../../modules/access/ProtectedRoute.tsx";
import LoginFlow from "../sign-in/LoginFlow.tsx";
import type { BrowserCapabilities } from "../../types.ts";
import { useSignOut } from "./useSignOut.ts";

const SIGN_OUT = "Sign out (spec)";

function SignOutButton(): ReactNode {
  const signOut = useSignOut();
  return (
    <button type="button" onClick={() => void signOut()}>
      {SIGN_OUT}
    </button>
  );
}

function CountMounts({ mounts, children }: { mounts: string[]; children: ReactNode }): ReactNode {
  useEffect(() => {
    mounts.push("login");
  }, [mounts]);
  return children;
}

describe("sign-out", () => {
  it("opens one login form only after the session has ended", async () => {
    seedSession({ user_id: 1, username: "admin", role: "admin" });
    let logoutReceived!: () => void;
    const logoutRequested = new Promise<void>((resolve) => {
      logoutReceived = resolve;
    });
    let releaseLogout!: () => void;
    const logoutReleased = new Promise<void>((resolve) => {
      releaseLogout = resolve;
    });
    worker.use(
      http.post("*/api/v1/auth/logout", async () => {
        logoutReceived();
        await logoutReleased;
        return new HttpResponse(null, { status: 204 });
      }),
      http.get("*/api/v1/auth/browser-capabilities", () =>
        HttpResponse.json({
          current_origin: "http://localhost:6680",
          passkey_available: false,
          passkey_unavailable_reason: "disabled",
          insecure_transport: false,
        } satisfies BrowserCapabilities),
      ),
    );
    const loginMounts: string[] = [];

    const screen = await renderWithProviders(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <SignOutButton />
              </ProtectedRoute>
            }
          />
          <Route
            path="/login"
            element={
              <CountMounts mounts={loginMounts}>
                <LoginFlow />
              </CountMounts>
            }
          />
        </Routes>
      </MemoryRouter>,
      { auth: true, router: false },
    );

    await screen.getByRole("button", { name: SIGN_OUT }).click();
    await logoutRequested;
    // The session is still authenticated while the server revokes it; a login
    // form mounted now would bounce back into the app and be replaced later.
    expect(loginMounts).toEqual([]);

    releaseLogout();
    const username = screen.getByLabelText(t("auth.login.username"), { exact: true });
    await username.fill("admin");

    await expect
      .element(screen.getByRole("button", { name: t("auth.login.continue") }))
      .toBeEnabled();
    await expect.element(username).toHaveValue("admin");
    expect(loginMounts).toEqual(["login"]);
  });
});
