import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import RequiredPasswordChangeFlow from "./RequiredPasswordChangeFlow";
import {
  clearRequiredPasswordChangeChallenge,
  storeRequiredPasswordChangeChallenge,
} from "../../state/passwordChangeChallenge";
import type { AuthResponse } from "../../types";

// Real internal chain: real form + browser-native validation, real AuthProvider,
// real generated $api mutation. Only the HTTP boundary is mocked, through MSW.
function renderFlow() {
  return renderWithProviders(
    <MemoryRouter initialEntries={["/password-change-required"]}>
      <Routes>
        <Route path="/password-change-required" element={<RequiredPasswordChangeFlow />} />
        <Route path="/login" element={<p>Login again</p>} />
        <Route path="/done" element={<p>Authenticated</p>} />
      </Routes>
    </MemoryRouter>,
    { router: false, auth: true },
  );
}

afterEach(() => {
  clearRequiredPasswordChangeChallenge();
});

describe("RequiredPasswordChangeFlow", () => {
  it("returns to login when the in-memory recovery token is absent", async () => {
    let completeCalls = 0;
    worker.use(
      http.post("/api/v1/auth/password-change/complete", () => {
        completeCalls += 1;
        return HttpResponse.json({});
      }),
    );

    const screen = await renderFlow();

    await expect.element(screen.getByText("Login again")).toBeVisible();
    expect(completeCalls).toBe(0);
  });

  it("exchanges the one-use token and authenticates only after completion", async () => {
    const authResponse = {
      token: "access-token",
      refreshToken: "refresh-token",
      user: { user_id: 1, username: "admin" },
    } satisfies AuthResponse;

    let receivedBody: unknown;
    worker.use(
      http.post("/api/v1/auth/password-change/complete", async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json(authResponse);
      }),
      http.get("/api/v1/auth/media-token", () =>
        HttpResponse.json({ token: "media-token", expires_at: new Date(Date.now() + 3_600_000).toISOString() }),
      ),
    );

    storeRequiredPasswordChangeChallenge({
      passwordChangeToken: "one-use-token",
      username: "admin",
      redirectTo: "/done",
    });

    const screen = await renderFlow();

    await screen
      .getByLabelText(t("auth.requiredPasswordChange.newPassword"), { exact: true })
      .fill("StrongPass123");
    await screen
      .getByLabelText(t("auth.requiredPasswordChange.confirmPassword"), { exact: true })
      .fill("StrongPass123");
    await screen.getByRole("button", { name: t("auth.requiredPasswordChange.submit") }).click();

    // Navigation to redirectTo is the observable proof the mutation resolved and
    // the session was completed through the real AuthProvider.
    await expect.element(screen.getByText("Authenticated")).toBeVisible();
    expect(receivedBody).toEqual({
      password_change_token: "one-use-token",
      new_password: "StrongPass123",
    });
  });
});
