import { validateRuntimeManifest } from "./manifestGuard";
import { verifyRuntimeManifestSignature } from "./signature";
import type {
  CatalogPluginSummary,
  PluginRevocationRecord,
  RuntimeManifestV1,
  StudioPluginPanel,
} from "./types";

const REVOCATION_CACHE_TTL_MS = 60_000;

let revocationCache: {
  fetchedAt: number;
  items: PluginRevocationRecord[];
} | null = null;

function getRegistryBaseUrl(): string {
  const raw = import.meta.env.VITE_PLUGIN_REGISTRY_URL?.trim();
  if (!raw) {
    throw new Error("VITE_PLUGIN_REGISTRY_URL is not configured");
  }
  return raw.replace(/\/$/, "");
}

function getAllowedCdnOrigin(): string | undefined {
  const raw = import.meta.env.VITE_PLUGIN_CDN_ORIGIN?.trim();
  if (!raw) return undefined;

  try {
    return new URL(raw).origin;
  } catch {
    throw new Error("VITE_PLUGIN_CDN_ORIGIN is not a valid URL");
  }
}

function buildRegistryUrl(path: string, query?: Record<string, string | undefined>): string {
  const base = getRegistryBaseUrl();
  const url = new URL(`${base}${path}`);

  if (query) {
    Object.entries(query).forEach(([key, value]) => {
      if (value) {
        url.searchParams.set(key, value);
      }
    });
  }

  return url.toString();
}

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Registry request failed (${response.status}): ${url}`);
  }
  return response.json() as Promise<T>;
}

export async function fetchPluginCatalog(
  panel?: StudioPluginPanel,
): Promise<CatalogPluginSummary[]> {
  const url = buildRegistryUrl("/v1/catalog", {
    panel,
  });
  return fetchJson<CatalogPluginSummary[]>(url);
}

export async function fetchPluginManifest(
  pluginId: string,
  version?: string,
): Promise<RuntimeManifestV1> {
  const encoded = encodeURIComponent(pluginId);
  const path = version
    ? `/v1/plugins/${encoded}/manifest/${encodeURIComponent(version)}`
    : `/v1/plugins/${encoded}/manifest`;
  return fetchJson<RuntimeManifestV1>(buildRegistryUrl(path));
}

export async function fetchPluginRevocations(force = false): Promise<PluginRevocationRecord[]> {
  if (
    !force &&
    revocationCache &&
    Date.now() - revocationCache.fetchedAt < REVOCATION_CACHE_TTL_MS
  ) {
    return revocationCache.items;
  }

  const url = buildRegistryUrl("/v1/revocations");
  const items = await fetchJson<PluginRevocationRecord[]>(url);
  revocationCache = {
    fetchedAt: Date.now(),
    items,
  };

  return items;
}

export function isManifestRevoked(
  manifest: RuntimeManifestV1,
  revocations: PluginRevocationRecord[],
): boolean {
  return revocations.some(
    (item) => item.active && item.id === manifest.id && item.version === manifest.version,
  );
}

export async function fetchAndVerifyManifest(
  pluginId: string,
  version?: string,
): Promise<RuntimeManifestV1> {
  const manifest = await fetchPluginManifest(pluginId, version);

  const validated = validateRuntimeManifest(manifest, {
    allowOrigin: getAllowedCdnOrigin(),
  });

  const signatureOk = await verifyRuntimeManifestSignature(validated);
  if (!signatureOk) {
    throw new Error(`Manifest signature verification failed for ${validated.id}@${validated.version}`);
  }

  const revocations = await fetchPluginRevocations();
  if (isManifestRevoked(validated, revocations)) {
    throw new Error(`Plugin ${validated.id}@${validated.version} has been revoked`);
  }

  return validated;
}
