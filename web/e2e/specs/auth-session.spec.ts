import type { APIRequestContext, APIResponse } from "playwright/test";
import { expect, test, type Workspace } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { t } from "../support/i18n";

type AuthResponse = {
  token: string;
  refreshToken: string;
  user: {
    user_id: number;
    username: string;
  };
};

async function expectAuth(response: APIResponse): Promise<AuthResponse> {
  if (!response.ok()) {
    expect(response.ok(), await response.text()).toBe(true);
  }
  const auth = (await response.json()) as AuthResponse;
  expect(auth.token).toBeTruthy();
  expect(auth.refreshToken).toBeTruthy();
  expect(auth.user.username).toBeTruthy();
  return auth;
}

async function login(request: APIRequestContext, workspace: Workspace): Promise<AuthResponse> {
  return expectAuth(
    await request.post("/api/v1/auth/login", {
      data: {
        username: workspace.username,
        password: workspace.password,
      },
    }),
  );
}

async function refresh(request: APIRequestContext, refreshToken: string): Promise<APIResponse> {
  return request.post("/api/v1/auth/refresh", {
    data: { refreshToken },
  });
}

test("@smoke refresh-token replay revokes every refresh session for the user", async ({
  request,
  workspace,
}) => {
  const firstDevice = await login(request, workspace);
  const secondDevice = await login(request, workspace);

  const rotatedFirstDevice = await expectAuth(await refresh(request, firstDevice.refreshToken));
  expect(rotatedFirstDevice.refreshToken).not.toBe(firstDevice.refreshToken);

  // The rotated token is usable before the predecessor is replayed.
  const currentUser = await request.get("/api/v1/auth/me", {
    headers: { authorization: `Bearer ${rotatedFirstDevice.token}` },
  });
  expect(currentUser.ok(), await currentUser.text()).toBe(true);

  // Reusing the predecessor is treated as theft. The API deliberately returns
  // the same 401 shape as any invalid token, while PostgreSQL revokes every
  // refresh token for this user.
  expect((await refresh(request, firstDevice.refreshToken)).status()).toBe(401);
  expect((await refresh(request, rotatedFirstDevice.refreshToken)).status()).toBe(401);
  expect((await refresh(request, secondDevice.refreshToken)).status()).toBe(401);

  // Short-lived access remains valid until expiry; the affected devices simply
  // cannot extend their sessions. A fresh password login starts a new family.
  const stillAuthenticated = await request.get("/api/v1/auth/me", {
    headers: { authorization: `Bearer ${secondDevice.token}` },
  });
  expect(stillAuthenticated.ok(), await stillAuthenticated.text()).toBe(true);

  const recovered = await login(request, workspace);
  expect((await refresh(request, recovered.refreshToken)).status()).toBe(200);
});

test("@smoke browser logout clears local credentials and revokes its refresh token", async ({
  page,
  request,
  workspace,
}) => {
  await new LoginPage(page).signIn(workspace.username, workspace.password);

  const accountButton = page.getByRole("button", { name: workspace.username });
  await accountButton.click();

  const refreshToken = await page.evaluate(
    () => Object.entries(window.localStorage).find(([key]) => /refresh.*token/i.test(key))?.[1],
  );
  if (!refreshToken) throw new Error("authenticated browser did not store a refresh credential");

  const logoutResponsePromise = page.waitForResponse(
    (incoming) =>
      incoming.request().method() === "POST" &&
      new URL(incoming.url()).pathname === "/api/v1/auth/logout",
  );
  await page.getByRole("button", { name: t("auth.logout"), exact: true }).click();

  const logoutResponse = await logoutResponsePromise;
  expect(logoutResponse.ok()).toBe(true);
  await expect(page).toHaveURL(/\/login$/);

  expect((await refresh(request, refreshToken)).status()).toBe(401);

  await expect
    .poll(() =>
      page.evaluate(() => Object.keys(window.localStorage).filter((key) => /token/i.test(key))),
    )
    .toEqual([]);
});
