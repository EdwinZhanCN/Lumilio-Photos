export interface VersionedStorageConfig {
  key: string;
  version: number;
  legacyKeys?: readonly string[];
}

export interface VersionedStorageReadResult {
  candidate: unknown | null;
  needsRewrite: boolean;
  source: "none" | "primary" | "legacy";
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

export function parseJSON(raw: string | null): unknown {
  if (!raw) return null;
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
}

export function extractVersionedCandidate(
  raw: unknown,
  version: number,
): { candidate: unknown | null; needsRewrite: boolean } {
  if (!isRecord(raw)) {
    return { candidate: null, needsRewrite: false };
  }

  const storedVersion = typeof raw.version === "number" ? raw.version : undefined;
  if (storedVersion === version && "data" in raw) {
    return { candidate: raw.data, needsRewrite: false };
  }

  if ("data" in raw) {
    return { candidate: raw.data, needsRewrite: true };
  }

  return { candidate: raw, needsRewrite: true };
}

export function readVersionedStorageCandidate(
  config: VersionedStorageConfig,
): VersionedStorageReadResult {
  if (typeof localStorage === "undefined") {
    return { candidate: null, needsRewrite: false, source: "none" };
  }

  const primary = extractVersionedCandidate(
    parseJSON(localStorage.getItem(config.key)),
    config.version,
  );
  if (primary.candidate !== null) {
    return {
      candidate: primary.candidate,
      needsRewrite: primary.needsRewrite,
      source: "primary",
    };
  }

  const legacyKeys = config.legacyKeys ?? [];
  for (const legacyKey of legacyKeys) {
    const legacyParsed = parseJSON(localStorage.getItem(legacyKey));
    if (legacyParsed === null) {
      continue;
    }

    const legacy = extractVersionedCandidate(legacyParsed, config.version);
    if (legacy.candidate !== null) {
      return {
        candidate: legacy.candidate,
        // Legacy key must be rewritten to the canonical key regardless of payload version.
        needsRewrite: true,
        source: "legacy",
      };
    }
  }

  return { candidate: null, needsRewrite: false, source: "none" };
}

export function writeVersionedStorageData<T>(
  key: string,
  version: number,
  data: T,
): void {
  if (typeof localStorage === "undefined") {
    return;
  }

  localStorage.setItem(
    key,
    JSON.stringify({
      version,
      data,
    }),
  );
}

export function removeStorageKeys(keys: readonly string[]): void {
  if (typeof localStorage === "undefined") {
    return;
  }

  keys.forEach((key) => {
    localStorage.removeItem(key);
  });
}
