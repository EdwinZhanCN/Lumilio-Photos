import process from "node:process";
import { isMFAInvalidError, loadBootstrapTOTP, nextTOTPCode, totpCode } from "./totp.ts";

const baseURL = process.env.LUMILIO_E2E_BASE_URL ?? "http://localhost:16657";
const username = process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin";
const password = process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!";

type RequestOptions = {
  method?: string;
  body?: string;
  headers?: Record<string, string>;
};

type Repository = {
  id: string;
  is_primary?: boolean;
};

async function request<T = Record<string, unknown>>(
  pathname: string,
  init: RequestOptions = {},
): Promise<T> {
  const response = await fetch(`${baseURL}${pathname}`, {
    ...init,
    headers: { "content-type": "application/json", ...init.headers },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new Error(
      `${init.method ?? "GET"} ${pathname}: ${response.status} ${JSON.stringify(body)}`,
    );
  return body as T;
}

const status = await request<{ admin_initialized: boolean }>("/api/v1/setup/status");

type AuthResponse = {
  token?: string;
  requires_mfa?: boolean;
  mfa_token?: string;
  mfa_methods?: string[];
};

async function loginBootstrap(): Promise<{ token: string }> {
  const login = await request<AuthResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
  if (login.token) return { token: login.token };
  if (!login.requires_mfa || !login.mfa_token) {
    throw new Error("bootstrap admin login did not return a session or an MFA challenge");
  }
  const bootstrapTOTP = loadBootstrapTOTP();
  if (!bootstrapTOTP || bootstrapTOTP.username !== username) {
    throw new Error("bootstrap admin requires MFA but the temporary E2E TOTP hand-off is missing");
  }
  const verify = (code: string) =>
    request<AuthResponse>("/api/v1/auth/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ mfa_token: login.mfa_token, code, method: "totp" }),
    });
  const code = totpCode(bootstrapTOTP.secret);
  let verified: AuthResponse;
  try {
    verified = await verify(code);
  } catch (error) {
    // The preceding first-admin regression may have consumed this counter.
    // Wait for the next counter only for the server's explicit replay error.
    if (!isMFAInvalidError(error)) throw error;
    verified = await verify(await nextTOTPCode(bootstrapTOTP.secret, code));
  }
  if (!verified.token) throw new Error("bootstrap admin MFA verification did not return a session");
  return { token: verified.token };
}

let auth: { token: string };
if (!status.admin_initialized) {
  auth = await request<{ token: string }>("/api/v1/auth/register/start", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
} else {
  auth = await loginBootstrap();
}
const headers = { authorization: `Bearer ${auth.token}` };
const repositories = await request<{ repositories: Repository[] }>("/api/v1/repositories", {
  headers,
}).catch(() => ({
  repositories: [],
}));
let primary: Repository | undefined = repositories.repositories?.find(
  (repository) => repository.is_primary,
);
if (!primary) {
  const created = await request<{ repository: Repository }>("/api/v1/repositories", {
    method: "POST",
    headers,
    body: JSON.stringify({
      name: "E2E Primary",
      role: "primary",
      storage_strategy: "flat",
      duplicate_handling: "rename",
    }),
  });
  primary = created.repository;
}

// Keep the E2E ML surface deliberate. The deterministic external fixture
// advertises semantic plus OCR for the isolated Agent reader scenario, but
// general seeding leaves OCR, face and BioCLIP workers disabled instead of
// adding inference work to every browser fixture.
await request("/api/v1/settings/system", {
  method: "PATCH",
  headers,
  body: JSON.stringify({
    llm: {
      agent_enabled: true,
      provider: "ollama",
      model_name: "lumilio-agent-e2e-v1",
      base_url: "http://agent-model-fixture:11434",
    },
    ml: {
      semantic_enabled: true,
      bioclip_enabled: false,
      ocr_enabled: false,
      face_enabled: false,
      video_semantic_enabled: true,
      video_max_frames: 8,
      video_long_threshold_seconds: 300,
      video_scene_threshold: 0.4,
    },
  }),
});

const runtimeSettings = await request<{
  llm?: {
    agent_enabled?: boolean;
    provider?: string;
    model_name?: string;
    base_url?: string;
    api_key_configured?: boolean;
  };
}>("/api/v1/settings/system", { headers });
if (
  runtimeSettings.llm?.agent_enabled !== true ||
  runtimeSettings.llm.provider !== "ollama" ||
  runtimeSettings.llm.model_name !== "lumilio-agent-e2e-v1" ||
  runtimeSettings.llm.base_url !== "http://agent-model-fixture:11434" ||
  runtimeSettings.llm.api_key_configured !== false
) {
  throw new Error(
    `keyless Agent runtime settings were not persisted: ${JSON.stringify(runtimeSettings.llm)}`,
  );
}

// Per-worker users, repositories and fixtures are provisioned by the
// worker-scoped `workspace` fixture, not here: this layer only has to leave a
// migrated database, a bootstrap admin, and the instance's single primary
// repository behind.
console.log(JSON.stringify({ username, primaryRepositoryId: primary.id }));
