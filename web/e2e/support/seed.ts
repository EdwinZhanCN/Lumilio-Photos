import process from "node:process";

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

let auth: { token: string };
if (!status.admin_initialized) {
  auth = await request<{ token: string }>("/api/v1/auth/register/start", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
} else {
  auth = await request<{ token: string }>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
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

// Keep the E2E ML surface deliberate. The deterministic external fixture only
// implements the two semantic contracts under test; unrelated OCR, face and
// BioCLIP workers stay disabled instead of accumulating retries.
await request("/api/v1/settings/system", {
  method: "PATCH",
  headers,
  body: JSON.stringify({
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

// Per-worker users, repositories and fixtures are provisioned by the
// worker-scoped `workspace` fixture, not here: this layer only has to leave a
// migrated database, a bootstrap admin, and the instance's single primary
// repository behind.
console.log(JSON.stringify({ username, primaryRepositoryId: primary.id }));
