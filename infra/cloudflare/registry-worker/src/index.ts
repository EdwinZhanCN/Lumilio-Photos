export interface Env {
  DB: D1Database;
  CACHE: KVNamespace;
  ARTIFACTS: R2Bucket;
  ALLOWED_ORIGIN: string;
}

interface CatalogRow {
  id: string;
  displayName: string;
  description: string | null;
  panel: string;
  latestVersion: string;
}

interface ReleaseRow {
  manifest_json: string;
}

interface RevocationRow {
  id: string;
  version: string;
  reason: string | null;
  active: number;
}

function buildCorsHeaders(request: Request, env: Env): HeadersInit {
  const requestOrigin = request.headers.get("Origin");
  const allowOrigin =
    requestOrigin && requestOrigin === env.ALLOWED_ORIGIN
      ? requestOrigin
      : env.ALLOWED_ORIGIN;

  return {
    "Access-Control-Allow-Origin": allowOrigin,
    "Access-Control-Allow-Methods": "GET, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
    Vary: "Origin",
  };
}

function jsonResponse(
  request: Request,
  env: Env,
  data: unknown,
  status = 200,
  extraHeaders: HeadersInit = {},
): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      ...buildCorsHeaders(request, env),
      ...extraHeaders,
    },
  });
}

function buildCacheKey(path: string, search: string): string {
  return `registry:${path}${search}`;
}

async function maybeGetCachedJson<T>(env: Env, key: string): Promise<T | null> {
  return env.CACHE.get<T>(key, "json");
}

async function cacheJson(env: Env, key: string, value: unknown, ttlSeconds: number): Promise<void> {
  await env.CACHE.put(key, JSON.stringify(value), { expirationTtl: ttlSeconds });
}

async function getCatalog(env: Env, panel: string | null): Promise<CatalogRow[]> {
  const sql = `
    SELECT
      p.plugin_id AS id,
      p.display_name AS displayName,
      p.description AS description,
      p.panel AS panel,
      r.version AS latestVersion
    FROM plugins p
    JOIN plugin_releases r
      ON p.plugin_id = r.plugin_id
     AND r.channel = 'stable'
     AND r.is_active = 1
    WHERE p.status = 'active'
      AND (?1 IS NULL OR p.panel = ?1)
    ORDER BY p.display_name ASC
  `;

  const rows = await env.DB.prepare(sql)
    .bind(panel)
    .all<CatalogRow>();

  return rows.results;
}

async function getManifest(
  env: Env,
  pluginId: string,
  version: string | null,
): Promise<Record<string, unknown> | null> {
  const sql = version
    ? `
      SELECT manifest_json
      FROM plugin_releases
      WHERE plugin_id = ?1
        AND version = ?2
      LIMIT 1
    `
    : `
      SELECT manifest_json
      FROM plugin_releases
      WHERE plugin_id = ?1
        AND channel = 'stable'
        AND is_active = 1
      ORDER BY created_at DESC
      LIMIT 1
    `;

  const query = version
    ? env.DB.prepare(sql).bind(pluginId, version)
    : env.DB.prepare(sql).bind(pluginId);

  const row = await query.first<ReleaseRow>();
  if (!row?.manifest_json) {
    return null;
  }

  return JSON.parse(row.manifest_json) as Record<string, unknown>;
}

async function getRevocations(env: Env): Promise<RevocationRow[]> {
  const sql = `
    SELECT
      plugin_id AS id,
      version,
      reason,
      active
    FROM plugin_revocations
    WHERE active = 1
    ORDER BY created_at DESC
  `;

  const rows = await env.DB.prepare(sql).all<RevocationRow>();
  return rows.results;
}

async function route(request: Request, env: Env): Promise<Response> {
  const url = new URL(request.url);

  if (request.method === "OPTIONS") {
    return new Response(null, {
      status: 204,
      headers: buildCorsHeaders(request, env),
    });
  }

  if (request.method !== "GET") {
    return jsonResponse(request, env, { error: "Method not allowed" }, 405);
  }

  const cacheKey = buildCacheKey(url.pathname, url.search);

  if (url.pathname === "/v1/catalog") {
    const panel = url.searchParams.get("panel");

    const cached = await maybeGetCachedJson<CatalogRow[]>(env, cacheKey);
    if (cached) {
      return jsonResponse(request, env, cached, 200, {
        "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
      });
    }

    const catalog = await getCatalog(env, panel);
    await cacheJson(env, cacheKey, catalog, 60);

    return jsonResponse(request, env, catalog, 200, {
      "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
    });
  }

  if (url.pathname === "/v1/revocations") {
    const cached = await maybeGetCachedJson<RevocationRow[]>(env, cacheKey);
    if (cached) {
      return jsonResponse(request, env, cached, 200, {
        "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
      });
    }

    const revocations = await getRevocations(env);
    await cacheJson(env, cacheKey, revocations, 60);

    return jsonResponse(request, env, revocations, 200, {
      "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
    });
  }

  const manifestVersionMatch = url.pathname.match(
    /^\/v1\/plugins\/([^/]+)\/manifest\/([^/]+)$/,
  );

  if (manifestVersionMatch) {
    const pluginId = decodeURIComponent(manifestVersionMatch[1]);
    const version = decodeURIComponent(manifestVersionMatch[2]);

    const cached = await maybeGetCachedJson<Record<string, unknown>>(env, cacheKey);
    if (cached) {
      return jsonResponse(request, env, cached, 200, {
        "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
      });
    }

    const manifest = await getManifest(env, pluginId, version);
    if (!manifest) {
      return jsonResponse(request, env, { error: "Manifest not found" }, 404);
    }

    await cacheJson(env, cacheKey, manifest, 60);

    return jsonResponse(request, env, manifest, 200, {
      "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
    });
  }

  const manifestActiveMatch = url.pathname.match(/^\/v1\/plugins\/([^/]+)\/manifest$/);
  if (manifestActiveMatch) {
    const pluginId = decodeURIComponent(manifestActiveMatch[1]);

    const cached = await maybeGetCachedJson<Record<string, unknown>>(env, cacheKey);
    if (cached) {
      return jsonResponse(request, env, cached, 200, {
        "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
      });
    }

    const manifest = await getManifest(env, pluginId, null);
    if (!manifest) {
      return jsonResponse(request, env, { error: "Manifest not found" }, 404);
    }

    await cacheJson(env, cacheKey, manifest, 60);

    return jsonResponse(request, env, manifest, 200, {
      "Cache-Control": "public, max-age=60, stale-while-revalidate=300",
    });
  }

  return jsonResponse(request, env, { error: "Not found" }, 404);
}

export default {
  fetch(request: Request, env: Env): Promise<Response> {
    return route(request, env);
  },
};
