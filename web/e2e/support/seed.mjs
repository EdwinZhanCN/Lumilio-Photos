import process from "node:process";

const baseURL = process.env.LUMILIO_E2E_BASE_URL ?? "http://127.0.0.1:16657";
const username = process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin";
const password = process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!";

async function request(pathname, init = {}) {
  const response = await fetch(`${baseURL}${pathname}`, {
    ...init,
    headers: { "content-type": "application/json", ...init.headers },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new Error(
      `${init.method ?? "GET"} ${pathname}: ${response.status} ${JSON.stringify(body)}`,
    );
  return body;
}

const status = await request("/api/v1/setup/status");
if (!status.database_initialized) await request("/api/v1/setup", { method: "POST", body: "{}" });

let auth;
if (!status.admin_initialized) {
  auth = await request("/api/v1/auth/register/start", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
} else {
  auth = await request("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}
const headers = { authorization: `Bearer ${auth.token}` };
const repositories = await request("/api/v1/repositories", { headers }).catch(() => ({
  repositories: [],
}));
let primary = repositories.repositories?.find((repository) => repository.is_primary);
if (!primary) {
  const created = await request("/api/v1/repositories", {
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

// Per-worker users, repositories and fixtures are provisioned by the
// worker-scoped `workspace` fixture, not here: this layer only has to leave a
// migrated database, a bootstrap admin, and the instance's single primary
// repository behind.
console.log(JSON.stringify({ username, primaryRepositoryId: primary.id }));
