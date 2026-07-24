import { readFileSync } from "node:fs";
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

type QueryResponse = {
  items?: Array<{
    asset?: {
      original_filename?: string;
    };
  }>;
};

type UploadResponse = {
  task_id: number;
  status: string;
};

type UploadJobStatus = {
  task_id: number;
  status: string;
  terminal: boolean;
  success: boolean;
  error?: string;
};

type UploadJobStatusResponse = {
  jobs?: UploadJobStatus[];
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

async function register(
  request: APIRequestContext,
  username: string,
  password: string,
): Promise<AuthResponse> {
  return expectAuth(
    await request.post("/api/v1/auth/register/start", {
      data: { username, password },
    }),
  );
}

async function uploadAsset(
  request: APIRequestContext,
  workspace: Workspace,
  filename: string,
): Promise<number> {
  const response = await request.post("/api/v1/assets", {
    headers: { authorization: `Bearer ${workspace.token}` },
    multipart: {
      repository_id: workspace.repositoryId,
      file: {
        name: filename,
        mimeType: "image/jpeg",
        buffer: readFileSync(workspace.authIsolationSource),
      },
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
  const upload = (await response.json()) as UploadResponse;
  expect(upload.status).toBe("processing");
  expect(upload.task_id).toBeGreaterThan(0);
  return upload.task_id;
}

async function uploadJobStatus(
  request: APIRequestContext,
  token: string,
  taskId: number,
): Promise<UploadJobStatus | undefined> {
  const response = await request.get(`/api/v1/assets/batch/jobs?task_ids=${taskId}`, {
    headers: { authorization: `Bearer ${token}` },
  });
  expect(response.ok(), await response.text()).toBe(true);
  const result = (await response.json()) as UploadJobStatusResponse;
  return result.jobs?.find((job) => job.task_id === taskId);
}

async function userCanSeeAsset(
  request: APIRequestContext,
  token: string,
  filename: string,
  repositoryId?: string,
): Promise<boolean> {
  const response = await request.post("/api/v1/assets/list", {
    headers: { authorization: `Bearer ${token}` },
    data: {
      query: filename,
      search_type: "filename",
      filter: repositoryId ? { repository_id: repositoryId } : {},
      pagination: { limit: 50, offset: 0 },
      stack_mode: "expanded",
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
  const result = (await response.json()) as QueryResponse;
  return result.items?.some((item) => item.asset?.original_filename === filename) ?? false;
}

async function expectSubjectLockout(
  requestAttempt: () => Promise<APIResponse>,
  expectedFailureStatus = 401,
): Promise<APIResponse> {
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const response = await requestAttempt();
    expect(response.status(), `attempt ${attempt}`).toBe(expectedFailureStatus);
  }

  const blocked = await requestAttempt();
  expect(blocked.status()).toBe(429);
  expect(Number(blocked.headers()["retry-after"])).toBeGreaterThanOrEqual(1);
  return blocked;
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

test("@auth-hardening authentication subjects lock out and recover without leaking credentials", async ({
  request,
  workspace,
}, testInfo) => {
  const username = `rate${testInfo.parallelIndex}x${Date.now()}`;
  await register(request, username, workspace.password);

  const blockedLogin = await expectSubjectLockout(() =>
    request.post("/api/v1/auth/login", {
      data: { username, password: "Wrong-Password-2026!" },
    }),
  );
  expect(await blockedLogin.text()).not.toContain(username);

  const correctWhileLocked = await request.post("/api/v1/auth/login", {
    data: { username, password: workspace.password },
  });
  expect(correctWhileLocked.status()).toBe(429);

  const retryAfterSeconds = Number(blockedLogin.headers()["retry-after"]);
  await new Promise((resolve) => setTimeout(resolve, retryAfterSeconds * 1_000 + 250));
  await expectAuth(
    await request.post("/api/v1/auth/login", {
      data: { username, password: workspace.password },
    }),
  );

  await expectSubjectLockout(() =>
    request.post("/api/v1/auth/passkeys/login/options", {
      data: { username },
    }),
  );
  const invalidPasskeyToken = `invalid-passkey-${testInfo.parallelIndex}-${Date.now()}`;
  await expectSubjectLockout(() =>
    request.post("/api/v1/auth/passkeys/login/verify", {
      data: {
        challenge_token: invalidPasskeyToken,
        credential: {},
      },
    }),
  );
  const invalidMFAToken = `invalid-mfa-${testInfo.parallelIndex}-${Date.now()}`;
  await expectSubjectLockout(() =>
    request.post("/api/v1/auth/mfa/verify", {
      data: {
        mfa_token: invalidMFAToken,
        code: "000000",
        method: "totp",
      },
    }),
  );
  const invalidRefreshToken = `invalid-refresh-${testInfo.parallelIndex}-${Date.now()}`;
  await expectSubjectLockout(() =>
    request.post("/api/v1/auth/refresh", {
      data: { refreshToken: invalidRefreshToken },
    }),
  );
});

test("@auth-hardening exhausted session clears user A before user B data loads", async ({
  page,
  request,
  workspace,
}, testInfo) => {
  test.setTimeout(150_000);
  const privateFilename = `auth-isolation-${testInfo.parallelIndex}-${Date.now()}.jpg`;
  const uploadTaskId = await uploadAsset(request, workspace, privateFilename);
  await expect
    .poll(
      async () => (await uploadJobStatus(request, workspace.token, uploadTaskId))?.terminal ?? false,
      {
        message: `${privateFilename} upload task should reach a terminal state for user A`,
        timeout: 120_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toBe(true);
  const completedUpload = await uploadJobStatus(request, workspace.token, uploadTaskId);
  expect(completedUpload?.success, completedUpload?.error ?? completedUpload?.status).toBe(true);
  expect(
    await userCanSeeAsset(request, workspace.token, privateFilename, workspace.repositoryId),
  ).toBe(true);

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/assets");
  await page
    .getByLabel(t("assets.assetsPageHeader.scope.title"))
    .filter({ visible: true })
    .selectOption({ label: workspace.repositoryName });
  await expect(page.getByLabel(new RegExp(privateFilename, "i"))).toBeVisible({
    timeout: 30_000,
  });

  const userASession = await page.evaluate(() => ({
    token: window.localStorage.getItem("auth_token"),
    refreshToken: window.localStorage.getItem("refresh_token"),
  }));
  if (!userASession.token || !userASession.refreshToken) {
    throw new Error("user A browser session was not stored");
  }
  await page.evaluate(() => {
    window.localStorage.setItem(
      "lumilio.settings.assets_state",
      JSON.stringify({ query: "user-a-private-state" }),
    );
    window.localStorage.setItem("assets_state_v1", "user-a-legacy-private-state");
  });

  const revoke = await request.post("/api/v1/auth/logout", {
    data: { refreshToken: userASession.refreshToken },
  });
  expect(revoke.ok(), await revoke.text()).toBe(true);
  await page.evaluate(() => window.localStorage.setItem("auth_token", "expired-access-token"));
  await page.reload();

  await expect(page).toHaveURL(/\/login$/, { timeout: 30_000 });
  await expect
    .poll(() =>
      page.evaluate(() =>
        Object.keys(window.localStorage).filter(
          (key) =>
            /token/i.test(key) ||
            key === "lumilio.settings.assets_state" ||
            key === "assets_state_v1",
        ),
      ),
    )
    .toEqual([]);

  const userB = `isolate${testInfo.parallelIndex}x${Date.now()}`;
  await register(request, userB, workspace.password);

  let releaseListRequest: (() => void) | undefined;
  const listRequestGate = new Promise<void>((resolve) => {
    releaseListRequest = resolve;
  });
  let markListRequestStarted: (() => void) | undefined;
  const listRequestStarted = new Promise<void>((resolve) => {
    markListRequestStarted = resolve;
  });
  await page.route("**/api/v1/assets/list", async (route) => {
    markListRequestStarted?.();
    await listRequestGate;
    await route.continue();
  });

  try {
    const listResponse = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === "/api/v1/assets/list",
    );
    await new LoginPage(page).signIn(userB, workspace.password, /\/assets$/);
    const userBToken = await page.evaluate(() => window.localStorage.getItem("auth_token"));
    if (!userBToken) throw new Error("user B browser session was not stored");
    expect(userBToken).not.toBe(userASession.token);
    await listRequestStarted;

    // While B's real response is held back, A's cached gallery must not render.
    await expect(page.getByLabel(new RegExp(privateFilename, "i"))).toHaveCount(0);

    releaseListRequest?.();
    const response = await listResponse;
    expect(response.ok(), await response.text()).toBe(true);
    await expect(page.getByLabel(new RegExp(privateFilename, "i"))).toHaveCount(0);
    expect(await userCanSeeAsset(request, userBToken, privateFilename)).toBe(false);
  } finally {
    releaseListRequest?.();
    await page.unroute("**/api/v1/assets/list");
  }
});
